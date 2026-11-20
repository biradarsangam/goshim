//go:build !podman

package goshim

import (
	"context"
	"errors"
	"strings"
	"testing"

	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestClosedPushSessionRejectsUpload(t *testing.T) {
	sessionCtx, cancel := context.WithCancel(context.Background())
	session := &PushSession{
		session: &pushSession{ctx: sessionCtx},
		cancel:  cancel,
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}

	_, err := session.Push(context.Background(), ocispec.Descriptor{
		MediaType: "application/octet-stream",
		Digest:    digest.FromString("payload"),
		Size:      int64(len("payload")),
	}, strings.NewReader("payload"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Push error = %v, want context.Canceled", err)
	}
}
