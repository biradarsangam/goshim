//go:build podman

package goshim

import (
	"context"

	digest "github.com/opencontainers/go-digest"
)

func backendLogin(ctx context.Context, registry, username, password string) error {
	return podmanLogin(ctx, registry, username, password)
}

func backendLogout(_ context.Context, registry string) error {
	return podmanLogout(registry)
}

func backendTransfer(ctx context.Context, source, destination string) (bool, digest.Digest, error) {
	return podmanTransfer(ctx, source, destination)
}
