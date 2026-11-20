package goshim

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	modelv1 "github.com/modelpack/model-spec/specs-go/v1"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestInspectModelLayoutReturnsValidatedModel(t *testing.T) {
	dir, ref := writeTestModelPack(t, modelv1.ArtifactTypeModelManifest, validTestModelConfig(t))

	got, err := InspectModelLayout(dir, ref)
	if err != nil {
		t.Fatalf("InspectModelLayout: %v", err)
	}
	if got.Model.Descriptor.Name != "test-model" {
		t.Fatalf("model name = %q, want test-model", got.Model.Descriptor.Name)
	}
	if got.Manifest.ArtifactType != modelv1.ArtifactTypeModelManifest {
		t.Fatalf("artifact type = %q", got.Manifest.ArtifactType)
	}
}

func TestInspectModelLayoutRejectsWrongArtifactType(t *testing.T) {
	dir, ref := writeTestModelPack(t, "application/vnd.example.other+json", validTestModelConfig(t))

	_, err := InspectModelLayout(dir, ref)
	if err == nil || !strings.Contains(err.Error(), "artifact type") {
		t.Fatalf("InspectModelLayout error = %v, want artifact type error", err)
	}
}

func TestInspectModelLayoutRejectsInvalidModelConfig(t *testing.T) {
	config := []byte(`{"descriptor":{},"modelfs":{"type":"layers","diffIds":["sha256:abc"]},"config":{},"unexpected":true}`)
	dir, ref := writeTestModelPack(t, modelv1.ArtifactTypeModelManifest, config)

	_, err := InspectModelLayout(dir, ref)
	if err == nil || !strings.Contains(err.Error(), "validate ModelPack config") {
		t.Fatalf("InspectModelLayout error = %v, want ModelPack schema validation error", err)
	}
}

func TestPushModelHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := PushModel(ctx, t.TempDir(), "registry.example/model:v1"); err != context.Canceled {
		t.Fatalf("PushModel error = %v, want context.Canceled", err)
	}
}

func TestPushModelRejectsInvalidModelPackBeforePush(t *testing.T) {
	config := []byte(`{"descriptor":{},"modelfs":{"type":"layers","diffIds":["sha256:abc"]},"config":{},"unexpected":true}`)
	dir, ref := writeTestModelPack(t, modelv1.ArtifactTypeModelManifest, config)

	if _, err := PushModel(context.Background(), dir, ref); err == nil || !strings.Contains(err.Error(), "validate ModelPack config") {
		t.Fatalf("PushModel error = %v, want ModelPack schema validation error", err)
	}
}

func TestVerifyModelBlobRejectsDigestMismatch(t *testing.T) {
	data := []byte("valid content")
	desc := ocispec.Descriptor{Digest: digest.FromString("different content"), Size: int64(len(data))}

	if err := verifyModelBlob(desc, data, "model config"); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("verifyModelBlob error = %v, want digest mismatch", err)
	}
}

func TestInspectModelLayoutRejectsInvalidConfigDigestBeforeReadingBlob(t *testing.T) {
	dir, ref := writeTestModelPack(t, modelv1.ArtifactTypeModelManifest, validTestModelConfig(t))
	manifestDesc, err := readManifestRef(dir, ref)
	if err != nil {
		t.Fatal(err)
	}
	manifestData, err := readBlob(dir, manifestDesc.Digest)
	if err != nil {
		t.Fatal(err)
	}
	var manifest ocispec.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.Config.Digest = digest.Digest("sha256:../../outside")
	manifestData, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestDesc, err = writeBlob(dir, ocispec.MediaTypeImageManifest, manifestData)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeManifestRef(dir, ref, manifestDesc); err != nil {
		t.Fatal(err)
	}

	_, err = InspectModelLayout(dir, ref)
	if err == nil || !strings.Contains(err.Error(), "invalid model config digest") {
		t.Fatalf("InspectModelLayout error = %v, want invalid digest error", err)
	}
}

func validTestModelConfig(t *testing.T) []byte {
	t.Helper()
	model := modelv1.Model{
		Descriptor: modelv1.ModelDescriptor{Name: "test-model"},
		ModelFS: modelv1.ModelFS{
			Type:    "layers",
			DiffIDs: []digest.Digest{digest.FromString("test-layer")},
		},
		Config: modelv1.ModelConfig{Format: "safetensors"},
	}
	data, err := json.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeTestModelPack(t *testing.T, artifactType string, configData []byte) (string, string) {
	t.Helper()
	dir := t.TempDir()
	ref := "registry.example/model:v1"
	if err := ensureLayout(dir); err != nil {
		t.Fatal(err)
	}

	configDesc, err := writeBlob(dir, modelv1.MediaTypeModelConfig, configData)
	if err != nil {
		t.Fatal(err)
	}
	layerDesc, err := writeBlob(dir, modelv1.MediaTypeModelWeightRaw, []byte("weights"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := ocispec.Manifest{
		MediaType:    ocispec.MediaTypeImageManifest,
		ArtifactType: artifactType,
		Config:       configDesc,
		Layers:       []ocispec.Descriptor{layerDesc},
	}
	manifest.SchemaVersion = 2
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestDesc, err := writeBlob(dir, ocispec.MediaTypeImageManifest, manifestData)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeManifestRef(dir, ref, manifestDesc); err != nil {
		t.Fatal(err)
	}
	return dir, ref
}
