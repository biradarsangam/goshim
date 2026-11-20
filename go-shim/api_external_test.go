package goshim_test

import (
	"context"
	"errors"
	"testing"

	goshim "github.com/biradarsangam/goshim/go-shim"
)

func TestPublicPackageCanBeImported(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := goshim.Pull(ctx, "registry.example/team/model:v1", t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Fatalf("Pull error = %v, want context.Canceled", err)
	}
}
