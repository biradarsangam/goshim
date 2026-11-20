// transfer_common.go — backend-agnostic helpers shared by both
// transfer_docker.go (!podman) and transfer_podman.go (podman).
package goshim

import (
	"strings"

	digest "github.com/opencontainers/go-digest"
)

// classifySource normalizes a registry source and rejects URI schemes that
// this registry-transfer backend does not handle.
func classifySource(ref string) (normalized string, err error) {
	if r, ok := cutAnyPrefix(ref, "hf://", "huggingface://"); ok {
		if strings.Count(r, "/") < 2 {
			r = "hf.co/" + r
		}
		return r, errHFNotHandledHere(normalizeTag(r))
	}
	normalized = normalizeTag(ref)
	return normalized, notHandledHere(normalized)
}

func cutAnyPrefix(s string, prefixes ...string) (string, bool) {
	for _, p := range prefixes {
		if r, ok := strings.CutPrefix(s, p); ok {
			return r, true
		}
	}
	return s, false
}

func shortDigest(d digest.Digest) string {
	h := d.Hex()
	if len(h) > 12 {
		return h[:12]
	}
	return h
}
