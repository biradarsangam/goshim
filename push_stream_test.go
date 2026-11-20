//go:build !podman

package goshim

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/vbauerster/mpb/v8"
)

func newTestPushSession(t *testing.T, ref string) *pushSession {
	t.Helper()
	ctx := context.Background()
	pusher, err := newResolver(ctx).Pusher(ctx, normalizeTag(ref))
	if err != nil {
		t.Fatalf("create pusher: %v", err)
	}
	return &pushSession{ctx: ctx, pusher: pusher}
}

func blobDescriptor(data []byte) ocispec.Descriptor {
	return ocispec.Descriptor{
		MediaType: "application/vnd.cncf.model.weight.v1.raw",
		Digest:    digest.FromBytes(data),
		Size:      int64(len(data)),
	}
}

func fetchBlob(t *testing.T, host, repo string, dgst digest.Digest) (string, int) {
	t.Helper()
	resp, err := http.Get("http://" + host + "/v2/" + repo + "/blobs/" + dgst.String())
	if err != nil {
		t.Fatalf("GET blob: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read blob: %v", err)
	}
	return string(body), resp.StatusCode
}

func TestPushSessionRejectsInvalidDescriptorBeforeUpload(t *testing.T) {
	session := &PushSession{session: &pushSession{ctx: context.Background()}}
	_, err := session.Push(context.Background(), ocispec.Descriptor{
		MediaType: "application/octet-stream",
		Digest:    "invalid",
		Size:      1,
	}, strings.NewReader("data"))
	if err == nil {
		t.Fatal("Push accepted an invalid digest")
	}
}

// pushLazy opens a source only when the destination lacks the blob, and
// always invokes its cleanup callback after an attempted open.
func TestPushLazyExistingBlobNeverOpensSource(t *testing.T) {
	log := &requestLog{}
	host := startLoggedRegistry(t, log, nil)
	session := newTestPushSession(t, host+"/org/model:v1")
	data := []byte("already there")
	desc := blobDescriptor(data)
	if _, err := pushBytes(session.ctx, session.pusher, desc, data); err != nil {
		t.Fatalf("seed push: %v", err)
	}
	session = newTestPushSession(t, host+"/org/model:v1")

	uploadsBefore := log.count("", "/v2/org/model/blobs/uploads/")
	opened := false
	exists, err := pushLazy(session.ctx, session.pusher, desc, func() (io.Reader, *mpb.Bar, func(), error) {
		opened = true
		return strings.NewReader(string(data)), nil, nil, nil
	})
	if err != nil || !exists {
		t.Fatalf("pushLazy = exists %t, err %v; want exists", exists, err)
	}
	if opened {
		t.Fatal("open was called for a blob the destination already has")
	}
	if n := log.count("", "/v2/org/model/blobs/uploads/") - uploadsBefore; n != 0 {
		t.Fatalf("saw %d upload requests, want 0", n)
	}
}

func TestPushLazyMissingBlobUploadsAndCleansUp(t *testing.T) {
	host := startRegistry(t)
	session := newTestPushSession(t, host+"/org/model:v1")
	data := []byte("not there yet")
	desc := blobDescriptor(data)

	cleaned := false
	exists, err := pushLazy(session.ctx, session.pusher, desc, func() (io.Reader, *mpb.Bar, func(), error) {
		return strings.NewReader(string(data)), nil, func() { cleaned = true }, nil
	})
	if err != nil || exists {
		t.Fatalf("pushLazy = exists %t, err %v; want an upload", exists, err)
	}
	if !cleaned {
		t.Fatal("cleanup was not called")
	}
	if got, status := fetchBlob(t, host, "org/model", desc.Digest); status != http.StatusOK || got != string(data) {
		t.Fatalf("registry blob = %d %q, want 200 %q", status, got, data)
	}
}

func TestPushLazyOpenErrorIsReturnedAndCleanedUp(t *testing.T) {
	host := startRegistry(t)
	session := newTestPushSession(t, host+"/org/model:v1")
	desc := blobDescriptor([]byte("never opened"))

	boom := errors.New("source unavailable")
	cleaned := false
	exists, err := pushLazy(session.ctx, session.pusher, desc, func() (io.Reader, *mpb.Bar, func(), error) {
		return nil, nil, func() { cleaned = true }, boom
	})
	if exists || !errors.Is(err, boom) {
		t.Fatalf("pushLazy = exists %t, err %v; want %v", exists, err, boom)
	}
	if !cleaned {
		t.Fatal("cleanup was not called after an open error")
	}
}
