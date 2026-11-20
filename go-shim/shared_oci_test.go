package goshim

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	digest "github.com/opencontainers/go-digest"
)

func TestWriteBlobConcurrent(t *testing.T) {
	dir := t.TempDir()
	data := bytes.Repeat([]byte("blob"), 1<<18)
	dgst := digest.FromBytes(data)
	start := make(chan struct{})

	var writes sync.WaitGroup
	for range 16 {
		writes.Go(func() {
			<-start
			if _, err := writeBlob(dir, "application/octet-stream", data); err != nil {
				t.Errorf("writeBlob: %v", err)
			}
		})
	}
	close(start)
	writes.Wait()

	got, err := readBlob(dir, dgst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("stored blob differs from the input")
	}
}

func TestWriteBlobCleansUpAfterFailedRename(t *testing.T) {
	dir := t.TempDir()
	data := []byte("blob")
	path := blobPath(dir, digest.FromBytes(data))
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := writeBlob(dir, "application/octet-stream", data); err == nil {
		t.Fatal("writeBlob replaced a directory")
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
		t.Fatalf("unexpected files after failed rename: %v", entries)
	}
}

func TestRetryStreamSkipsAttemptWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := retryStream(ctx, "upload", nil, func() error {
		t.Fatal("attempt called with a canceled context")
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestRetryStreamReportsActualAttemptCount(t *testing.T) {
	rejected := errors.New("registry rejected upload")
	attempts := 0
	err := retryStream(t.Context(), "upload", func(err error) bool {
		return errors.Is(err, rejected)
	}, func() error {
		attempts++
		return rejected
	})
	if attempts != 1 || !errors.Is(err, rejected) || !strings.Contains(err.Error(), "failed on attempt 1:") {
		t.Fatalf("attempts = %d, error = %v", attempts, err)
	}
}
