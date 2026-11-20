package goshim

import (
	"context"
	"errors"
	"strings"
	"testing"

	digest "github.com/opencontainers/go-digest"
)

func TestHighLevelOperationsHonorCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := Pull(ctx, "registry.example/team/model:v1", t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Errorf("Pull error = %v, want context.Canceled", err)
	}
	if _, err := Push(ctx, t.TempDir(), "registry.example/team/model:v1"); !errors.Is(err, context.Canceled) {
		t.Errorf("Push error = %v, want context.Canceled", err)
	}
	if _, err := Transfer(ctx, "source", "destination"); !errors.Is(err, context.Canceled) {
		t.Errorf("Transfer error = %v, want context.Canceled", err)
	}
	if err := Login(ctx, "registry.example", "user", "password"); !errors.Is(err, context.Canceled) {
		t.Errorf("Login error = %v, want context.Canceled", err)
	}
}

func TestVerifyRequiresTrustPolicyBeforeNetworkAccess(t *testing.T) {
	target := digest.FromString("manifest").String()
	_, err := Verify(context.Background(), "registry.example/team/model:v1", VerifyOptions{
		Digest: target,
	})
	if err == nil || !strings.Contains(err.Error(), "no trusted keys") {
		t.Fatalf("Verify error = %v, want missing trust policy", err)
	}
}
