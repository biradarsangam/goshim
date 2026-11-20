//go:build !podman

package goshim

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/containerd/containerd/v2/core/remotes"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type pushSession struct {
	ctx    context.Context
	pusher remotes.Pusher
}

// PushSession reuses a registry pusher across streamed blob uploads.
// It is available with the default Docker-compatible backend.
type PushSession struct {
	session *pushSession
	cancel  context.CancelFunc
}

// NewPushSession creates a reusable upload session for destination.
func NewPushSession(ctx context.Context, destination string) (*PushSession, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sessionCtx, cancel := context.WithCancel(ctx)
	resolver := newResolver(sessionCtx)
	pusher, err := resolver.Pusher(sessionCtx, normalizeTag(destination))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create pusher: %w", err)
	}
	return &PushSession{
		session: &pushSession{ctx: sessionCtx, pusher: pusher},
		cancel:  cancel,
	}, nil
}

// Push uploads the descriptor from src. It returns true when the destination
// already contains the blob; in that case src is not read.
func (s *PushSession) Push(ctx context.Context, desc ocispec.Descriptor, src io.Reader) (bool, error) {
	if s == nil || s.session == nil || s.session.ctx == nil {
		return false, errors.New("push session is not initialized")
	}
	if err := s.session.ctx.Err(); err != nil {
		return false, err
	}
	if src == nil {
		return false, errors.New("push source is nil")
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if desc.MediaType == "" {
		return false, errors.New("descriptor has no media type")
	}
	if err := desc.Digest.Validate(); err != nil {
		return false, fmt.Errorf("invalid descriptor digest: %w", err)
	}
	if desc.Size < 0 {
		return false, errors.New("descriptor has a negative size")
	}
	uploadCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(s.session.ctx, cancel)
	defer func() {
		stop()
		cancel()
	}()
	if err := uploadCtx.Err(); err != nil {
		return false, err
	}
	return pushStreamLazy(uploadCtx, s.session.pusher, desc, nil, func() (io.ReadCloser, error) {
		return io.NopCloser(src), nil
	})
}

// Close cancels the session context. It is safe to call more than once.
func (s *PushSession) Close() error {
	if s == nil || s.cancel == nil {
		return nil
	}
	s.cancel()
	return nil
}
