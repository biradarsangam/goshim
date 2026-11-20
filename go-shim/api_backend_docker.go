//go:build !podman

package goshim

import (
	"context"

	digest "github.com/opencontainers/go-digest"
)

func backendLogin(_ context.Context, registry, username, password string) error {
	return dockerLogin(registry, username, password)
}

func backendLogout(_ context.Context, registry string) error {
	return dockerLogout(registry)
}

func backendTransfer(ctx context.Context, source, destination string) (bool, digest.Digest, error) {
	return dockerTransfer(ctx, source, destination)
}
