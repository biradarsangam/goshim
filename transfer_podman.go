//go:build podman

// transfer_podman.go implements transfers with the Podman
// containers/image backend.
//
// For an OCI registry source, this is a one-line reuse of go.podman.io/image's
// own `copy.Image`: hand it a docker://source and a docker://destination
// reference directly, with no local OCI layout in between at all.
// copy.Image already streams each blob straight from source to destination
// (it reads the source manifest first, so every blob's digest/size is known
// up front, then opens a GetBlob reader and a PutBlob writer for each one —
// see copy/copy.go upstream); there's nothing goshim needs to add on top
// for that case.
//
// Only registry references are handled here; other sources should be
// staged into an OCI layout and uploaded with Push.
package goshim

import (
	"context"
	"fmt"
	"os"

	digest "github.com/opencontainers/go-digest"
	"go.podman.io/image/v5/copy"
	"go.podman.io/image/v5/manifest"
	"go.podman.io/image/v5/transports/alltransports"
)

// podmanTransfer returns whether anything was actually pushed to
// destination — see dockerTransfer's doc comment (transfer_docker.go) for
// why this matters for a re-run against an unchanged source, and for why
// the pushed manifest digest comes back alongside it.
func podmanTransfer(ctx context.Context, source, destination string) (changed bool, pushed digest.Digest, err error) {
	// See transfer_docker.go's dockerTransfer for why a tagless
	// destination must default to :latest explicitly here.
	destination = normalizeTag(destination)
	normalized, err := classifySource(source)
	if err != nil {
		return false, "", err
	}
	return podmanTransferOCI(ctx, normalized, destination)
}

// podmanTransferOCI streams directly between two registries via
// go.podman.io/image's copy.Image — no local OCI layout involved. Routed
// through copyImageWithProgress (backend_podman.go), rather than a bare
// copy.Image call, purely so its ProgressEventDone/Skipped events can
// report whether anything was actually transferred — see that function's
// doc comment.
func podmanTransferOCI(ctx context.Context, source, destination string) (changed bool, pushed digest.Digest, err error) {
	srcStr := "docker://" + source
	srcRef, err := alltransports.ParseImageName(srcStr)
	if err != nil {
		return false, "", fmt.Errorf("parse src ref %q: %w", srcStr, err)
	}
	dstStr := "docker://" + destination
	dstRef, err := alltransports.ParseImageName(dstStr)
	if err != nil {
		return false, "", fmt.Errorf("parse dst ref %q: %w", dstStr, err)
	}

	pctx, err := insecurePolicy()
	if err != nil {
		return false, "", fmt.Errorf("policy context: %w", err)
	}
	defer pctx.Destroy()

	// Transfer progress is rendered directly, so there is no per-reference
	// progress key to credit these bytes to.
	copied, err := copyImageWithProgress(ctx, pctx, dstRef, srcRef, "Transferring", "Transferred", &copy.Options{
		ReportWriter: os.Stderr,
		SourceCtx:    registrySourceCtx(ctx),
	}, &changed, "")
	if err != nil {
		return false, "", fmt.Errorf("transfer image: %w", err)
	}
	// copy.Image hands back the manifest it wrote to the destination, so
	// this is the digest of what actually landed rather than a re-resolve
	// of the tag — see dockerTransfer for why that matters to
	// --sign-key. An empty manifest would hash to the digest of "",
	// which would be signed and published at a tag naming nothing.
	if len(copied) == 0 {
		return false, "", fmt.Errorf("transfer %s: destination reported no manifest", destination)
	}
	// manifest.Digest, not digest.FromBytes: a Docker schema1 manifest
	// is addressed by a digest over its canonicalized payload, not its
	// raw bytes. Matches resolveManifestDigest.
	pushed, err = manifest.Digest(copied)
	if err != nil {
		return false, "", fmt.Errorf("digest transferred manifest: %w", err)
	}
	return changed, pushed, nil
}
