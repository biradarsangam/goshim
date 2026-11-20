// classify.go validates and normalizes registry references before
// registry operations. It deliberately does not probe the network to
// determine whether a reference points to a registry.
package goshim

import (
	"fmt"
	"strings"
)

// errHFNotHandledHere reports a HuggingFace reference passed to the
// registry-only transfer operation.
func errHFNotHandledHere(ref string) error {
	return fmt.Errorf(
		"%s: HuggingFace references are not supported by the registry transfer backend", ref)
}

// errSourceNotHandledHere reports a non-registry source passed to the
// registry-only transfer operation.
func errSourceNotHandledHere(ref string) error {
	return fmt.Errorf(
		"%s: non-registry sources are not supported by the registry transfer backend", ref)
}

// isKnownHFHost returns true for the known HuggingFace-compatible hosts.
func isKnownHFHost(host string) bool {
	switch host {
	case "hf.co", "huggingface.co", "modelscope.cn":
		return true
	}
	return false
}

// sourceSchemes are the URI schemes src/sources owns. A reference
// carrying one of these must never reach the registry client below.
var sourceSchemes = []string{"ms://", "modelscope://", "ngc://", "s3://", "gs://"}

// notHandledHere reports a reference that is not a registry reference.
func notHandledHere(ref string) error {
	for _, scheme := range sourceSchemes {
		if strings.HasPrefix(ref, scheme) {
			return errSourceNotHandledHere(ref)
		}
	}
	if strings.HasPrefix(ref, "/") {
		return errSourceNotHandledHere(ref)
	}
	if isKnownHFHost(strings.SplitN(ref, "/", 2)[0]) {
		return errHFNotHandledHere(ref)
	}
	return nil
}

// classifyPullRef normalizes ref for both backends' pullToLayout and
// rejects anything that isn't an OCI registry reference.
func classifyPullRef(ref string) (normalizedRef string, err error) {
	ref = normalizeTag(ref)
	return ref, notHandledHere(ref)
}

// normalizeTag appends ":latest" unless ref already carries a tag or a
// digest. A ":" only starts a tag if it comes after the last "/", so a
// registry port ("host:5000/owner/repo") isn't mistaken for one.
func normalizeTag(ref string) string {
	if strings.LastIndex(ref, ":") <= strings.LastIndex(ref, "/") {
		return ref + ":latest"
	}
	return ref
}
