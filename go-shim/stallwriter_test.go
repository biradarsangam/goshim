package goshim

import (
	"context"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/containerd/containerd/v2/core/content"
	digest "github.com/opencontainers/go-digest"
)

// blockingWriter is a content.Writer whose Write never returns on its
// own, simulating exactly the observed real-world failure: a registry
// PUT (an io.Pipe.Write, in the real code) that goes quiet mid-upload
// and never completes or fails by itself.
type blockingWriter struct{ unblock chan struct{} }

func (blockingWriter) Digest() digest.Digest                                              { return "" }
func (blockingWriter) Commit(context.Context, int64, digest.Digest, ...content.Opt) error { return nil }
func (blockingWriter) Status() (content.Status, error)                                    { return content.Status{}, nil }
func (blockingWriter) Truncate(int64) error                                               { return nil }
func (blockingWriter) Close() error                                                       { return nil }
func (w blockingWriter) Write(p []byte) (int, error) {
	<-w.unblock
	return len(p), nil
}

func TestStallWriterCancelsOnBlockedWrite(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		unblock := make(chan struct{})
		sw := newStallWriter(blockingWriter{unblock: unblock}, time.Second, cancel)
		defer func() {
			close(unblock)
			synctest.Wait()
			sw.stop()
		}()

		go sw.Write([]byte("hello"))
		time.Sleep(2 * time.Second)
		if ctx.Err() != context.Canceled {
			t.Fatal("stall timer did not cancel the blocked write")
		}
	})
}

func TestStallWriterResetsOnProgress(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		unblock := make(chan struct{})
		close(unblock)
		sw := newStallWriter(blockingWriter{unblock: unblock}, time.Second, cancel)
		defer sw.stop()
		time.Sleep(500 * time.Millisecond)
		if _, err := sw.Write([]byte("hello")); err != nil {
			t.Fatalf("Write: %v", err)
		}
		checkStallReset(t, ctx)
	})
}

func TestStallReaderResetsOnProgress(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		sr := newStallReader(strings.NewReader("hello"), time.Second, cancel)
		defer sr.stop()
		time.Sleep(500 * time.Millisecond)
		if _, err := sr.Read(make([]byte, 1)); err != nil {
			t.Fatalf("Read: %v", err)
		}
		checkStallReset(t, ctx)
	})
}

func checkStallReset(t *testing.T, ctx context.Context) {
	t.Helper()
	time.Sleep(750 * time.Millisecond)
	if err := ctx.Err(); err != nil {
		t.Fatalf("stall timer fired before the extended deadline: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
	if ctx.Err() != context.Canceled {
		t.Fatal("stall timer did not fire after the extended deadline")
	}
}
