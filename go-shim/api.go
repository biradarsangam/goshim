package goshim

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Login stores credentials for a registry using the configured backend.
func Login(ctx context.Context, registry, username, password string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return backendLogin(ctx, registry, username, password)
}

// Logout removes credentials for a registry using the configured backend.
func Logout(ctx context.Context, registry string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return backendLogout(ctx, registry)
}

// Push uploads the manifest selected by ref from an OCI image layout.
// It returns false when the destination already contains all of its content.
func Push(ctx context.Context, layoutDir, ref string) (changed bool, err error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	progressReset(ref, "retrieving manifest")
	defer progressDone(ref)
	return pushToRegistry(ctx, layoutDir, ref)
}

// Pull downloads ref into an OCI image layout.
func Pull(ctx context.Context, ref, layoutDir string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	progressReset(ref, "pulling manifest")
	defer progressDone(ref)
	return pullToLayout(ctx, ref, layoutDir)
}

// Inspect fetches the raw manifest JSON for ref.
func Inspect(ctx context.Context, ref string) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := fetchManifestRaw(ctx, ref)
	if err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), data...), nil
}

// TransferResult describes what a completed transfer changed.
type TransferResult struct {
	Changed bool          `json:"changed"`
	Digest  digest.Digest `json:"digest,omitempty"`
}

// Transfer copies a registry image from source to destination without
// materializing the image in a persistent local store.
func Transfer(ctx context.Context, source, destination string) (TransferResult, error) {
	if err := ctx.Err(); err != nil {
		return TransferResult{}, err
	}
	changed, pushed, err := backendTransfer(ctx, source, destination)
	if err != nil {
		return TransferResult{}, err
	}
	result := TransferResult{Changed: changed}
	if pushed != "" {
		result.Digest = pushed
	}
	return result, nil
}

// PushArtifact attaches subject to the manifest in layoutDir and publishes
// the resulting artifact to ref.
func PushArtifact(ctx context.Context, layoutDir, ref string, subject ocispec.Descriptor) (ocispec.Descriptor, error) {
	if err := ctx.Err(); err != nil {
		return ocispec.Descriptor{}, err
	}
	if err := validateSubject(subject); err != nil {
		return ocispec.Descriptor{}, err
	}
	progressReset(ref, "retrieving manifest")
	defer progressDone(ref)

	artifact, err := setManifestSubject(layoutDir, ref, subject)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	if _, err := pushDescriptorToRegistry(ctx, layoutDir, ref, artifact); err != nil {
		return ocispec.Descriptor{}, err
	}
	if err := publishFallbackReferrer(ctx, ref, subject.Digest, artifact); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("publish referrer index: %w", err)
	}
	return artifact, nil
}

// Referrers lists artifacts attached to subjectDigest in ref's repository.
func Referrers(ctx context.Context, ref, subjectDigest string) ([]ocispec.Descriptor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dgst, err := digest.Parse(subjectDigest)
	if err != nil {
		return nil, fmt.Errorf("parse subject digest: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	refs, _, err := fetchReferrers(ctx, ref, dgst)
	return refs, err
}

// SetRegistryMirrors replaces the mirror configuration. An empty map clears it.
func SetRegistryMirrors(mirrors map[string][]string) error {
	data, err := json.Marshal(mirrors)
	if err != nil {
		return fmt.Errorf("encode registry mirrors: %w", err)
	}
	return setRegistryMirrors(string(data))
}

// ProgressSnapshot reports byte progress for a currently active operation.
type ProgressSnapshot struct {
	Status    string `json:"status"`
	Total     int64  `json:"total"`
	Completed int64  `json:"completed"`
}

// Progress returns the current byte-progress snapshot for key. It returns a
// zero-value snapshot when no operation with that key is active.
func Progress(key string) ProgressSnapshot {
	progressState.mu.Lock()
	defer progressState.mu.Unlock()
	if e, ok := progressState.entries[key]; ok {
		return ProgressSnapshot{Status: e.status, Total: e.total, Completed: e.completed}
	}
	return ProgressSnapshot{}
}

// ResolveDigest resolves ref to its current manifest digest.
func ResolveDigest(ctx context.Context, ref string) (digest.Digest, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := notHandledHere(ref); err != nil {
		return "", err
	}
	resolved, err := resolveManifestDigest(ctx, normalizeTag(ref))
	if err != nil {
		return "", markIfMissing(err, ref)
	}
	return resolved, nil
}

// IdentityPolicy defines the issuer, subject, and Sigstore TrustedRoot used
// for keyless signature verification.
type IdentityPolicy struct {
	Issuer      string `json:"issuer"`
	Subject     string `json:"subject"`
	TrustedRoot string `json:"trustedRoot"`
}

// VerifyOptions configures signature verification.
type VerifyOptions struct {
	Digest   string
	KeyPaths []string
	Claims   string
	Identity *IdentityPolicy
}

// SignatureMatch describes a signature accepted by the verification policy.
type SignatureMatch struct {
	Signer   string `json:"signer"`
	KeyPath  string `json:"keyPath"`
	Identity string `json:"identity"`
}

// VerificationReport describes the signatures found and verified for a
// manifest. A false Verified value is a successful verification result;
// errors indicate verification could not be completed.
type VerificationReport struct {
	Reference       string           `json:"reference"`
	Digest          string           `json:"digest"`
	Verified        bool             `json:"verified"`
	SignaturesFound int              `json:"signaturesFound"`
	Matches         []SignatureMatch `json:"matches"`
	Reason          string           `json:"reason"`
}

// Verify checks signatures for ref against the supplied key and identity
// policy. Supplying options.Digest avoids resolving a moving tag again.
func Verify(ctx context.Context, ref string, options VerifyOptions) (VerificationReport, error) {
	if err := ctx.Err(); err != nil {
		return VerificationReport{}, err
	}
	ctx = withoutMirrors(ctx)
	var identity *identityPolicy
	if options.Identity != nil {
		identity = &identityPolicy{
			Issuer:      options.Identity.Issuer,
			Subject:     options.Identity.Subject,
			TrustedRoot: options.Identity.TrustedRoot,
		}
	}
	target, err := targetDigest(ctx, ref, options.Digest)
	if err != nil {
		return VerificationReport{}, err
	}
	report, err := verifySignatures(ctx, ref, target, options.KeyPaths, identity, options.Claims)
	if err != nil {
		return VerificationReport{}, err
	}
	out := VerificationReport{
		Reference:       report.Reference,
		Digest:          report.Digest,
		Verified:        report.Verified,
		SignaturesFound: report.SignaturesFound,
		Reason:          report.Reason,
		Matches:         make([]SignatureMatch, 0, len(report.Matches)),
	}
	for _, match := range report.Matches {
		out.Matches = append(out.Matches, SignatureMatch{
			Signer:   match.Signer,
			KeyPath:  match.KeyPath,
			Identity: match.Identity,
		})
	}
	return out, nil
}

// Sign signs the selected manifest with a PEM private key and publishes its
// cosign-format signature. An empty digest resolves the current ref first.
func Sign(ctx context.Context, ref, givenDigest, keyPath string, password []byte) (digest.Digest, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if keyPath == "" {
		return "", errors.New("no signing key given")
	}
	ctx = withoutMirrors(ctx)
	target, err := targetDigest(ctx, ref, givenDigest)
	if err != nil {
		return "", err
	}
	if err := signManifest(ctx, ref, target, keyPath, password); err != nil {
		return "", err
	}
	return target, nil
}
