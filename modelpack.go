package goshim

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	modelschema "github.com/modelpack/model-spec/schema"
	modelv1 "github.com/modelpack/model-spec/specs-go/v1"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ModelArtifact contains the ModelPack manifest and its validated model
// configuration.
type ModelArtifact struct {
	Manifest ocispec.Manifest `json:"manifest"`
	Model    modelv1.Model    `json:"model"`
}

// InspectModelLayout reads and validates a ModelPack artifact in an OCI
// layout without downloading or reading its model layers.
func InspectModelLayout(layoutDir, ref string) (ModelArtifact, error) {
	manifestDesc, err := readManifestRef(layoutDir, ref)
	if err != nil {
		return ModelArtifact{}, fmt.Errorf("read manifest reference %q: %w", ref, err)
	}
	if manifestDesc.MediaType != ocispec.MediaTypeImageManifest {
		return ModelArtifact{}, fmt.Errorf("manifest %q has media type %q, want %q", ref, manifestDesc.MediaType, ocispec.MediaTypeImageManifest)
	}
	if err := manifestDesc.Digest.Validate(); err != nil {
		return ModelArtifact{}, fmt.Errorf("invalid manifest digest for %q: %w", ref, err)
	}

	manifestData, err := readBlob(layoutDir, manifestDesc.Digest)
	if err != nil {
		return ModelArtifact{}, fmt.Errorf("read manifest blob for %q: %w", ref, err)
	}
	if err := verifyModelBlob(manifestDesc, manifestData, "manifest"); err != nil {
		return ModelArtifact{}, fmt.Errorf("validate manifest for %q: %w", ref, err)
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return ModelArtifact{}, fmt.Errorf("parse manifest for %q: %w", ref, err)
	}
	if manifest.MediaType != ocispec.MediaTypeImageManifest {
		return ModelArtifact{}, fmt.Errorf("manifest %q has media type %q, want %q", ref, manifest.MediaType, ocispec.MediaTypeImageManifest)
	}
	if manifest.ArtifactType != modelv1.ArtifactTypeModelManifest {
		return ModelArtifact{}, fmt.Errorf("manifest %q has artifact type %q, want %q", ref, manifest.ArtifactType, modelv1.ArtifactTypeModelManifest)
	}
	if manifest.Config.MediaType != modelv1.MediaTypeModelConfig {
		return ModelArtifact{}, fmt.Errorf("model config has media type %q, want %q", manifest.Config.MediaType, modelv1.MediaTypeModelConfig)
	}
	if err := manifest.Config.Digest.Validate(); err != nil {
		return ModelArtifact{}, fmt.Errorf("invalid model config digest for %q: %w", ref, err)
	}

	configData, err := readBlob(layoutDir, manifest.Config.Digest)
	if err != nil {
		return ModelArtifact{}, fmt.Errorf("read model config for %q: %w", ref, err)
	}
	if err := verifyModelBlob(manifest.Config, configData, "model config"); err != nil {
		return ModelArtifact{}, fmt.Errorf("validate model config for %q: %w", ref, err)
	}
	if err := modelschema.Validator(modelv1.MediaTypeModelConfig).Validate(bytes.NewReader(configData)); err != nil {
		return ModelArtifact{}, fmt.Errorf("validate ModelPack config for %q: %w", ref, err)
	}

	var model modelv1.Model
	if err := json.Unmarshal(configData, &model); err != nil {
		return ModelArtifact{}, fmt.Errorf("decode ModelPack config for %q: %w", ref, err)
	}
	return ModelArtifact{Manifest: manifest, Model: model}, nil
}

// PushModel validates a ModelPack artifact in an OCI layout before pushing it.
func PushModel(ctx context.Context, layoutDir, ref string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if _, err := InspectModelLayout(layoutDir, ref); err != nil {
		return false, err
	}
	return Push(ctx, layoutDir, ref)
}

// PullModel pulls an OCI artifact into a layout and validates it as a
// ModelPack artifact. The downloaded layout remains available if validation
// fails.
func PullModel(ctx context.Context, ref, layoutDir string) (ModelArtifact, error) {
	if err := Pull(ctx, ref, layoutDir); err != nil {
		return ModelArtifact{}, err
	}
	return InspectModelLayout(layoutDir, ref)
}

func verifyModelBlob(desc ocispec.Descriptor, data []byte, kind string) error {
	if desc.Size != int64(len(data)) {
		return fmt.Errorf("%s size mismatch: descriptor says %d bytes, got %d", kind, desc.Size, len(data))
	}
	if err := desc.Digest.Validate(); err != nil {
		return fmt.Errorf("invalid %s digest: %w", kind, err)
	}
	actual := desc.Digest.Algorithm().FromBytes(data)
	if actual != desc.Digest {
		return fmt.Errorf("%s digest mismatch: descriptor says %s, got %s", kind, desc.Digest, actual)
	}
	return nil
}
