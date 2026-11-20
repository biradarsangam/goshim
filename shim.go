// Package goshim provides registry, OCI layout, transfer, and signing
// operations for Go applications.
package goshim

import "strings"

// ImageNotFoundError marks a registry response as indicating that the
// requested image or manifest does not exist.
type ImageNotFoundError struct{ Err error }

func (e ImageNotFoundError) Error() string { return e.Err.Error() }
func (e ImageNotFoundError) Unwrap() error { return e.Err }

// isDockerHubRef reports whether ref is on Docker Hub, which answers an
// anonymous pull of a missing repository as it does a private one. Another
// registry's anonymous denial is an auth failure, not a missing image.
func isDockerHubRef(ref string) bool {
	host, _, _ := strings.Cut(ref, "/")
	switch host {
	case "docker.io", "index.docker.io", "registry-1.docker.io":
		return true
	}
	return false
}

// markImageNotFound is err, marked as the registry's "no such image".
func markImageNotFound(err error) error { return ImageNotFoundError{Err: err} }

// markIfMissing is err, marked as a missing image if the registry said so
// about ref.
func markIfMissing(err error, ref string) error {
	if isImageNotFound(err, ref) {
		return markImageNotFound(err)
	}
	return err
}
