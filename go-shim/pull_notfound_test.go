package goshim

import (
	"errors"
	"fmt"
	"testing"
)

// Marked not-found errors retain their message and unwrap to the cause.
func TestMarkedImageNotFoundError(t *testing.T) {
	cause := errors.New("resolve: docker.io/ai/nope:1b: not found")
	var notFound ImageNotFoundError
	if errors.As(cause, &notFound) {
		t.Fatalf("unmarked error %v was classified as not found", cause)
	}

	marked := markImageNotFound(cause)
	wrapped := fmt.Errorf("wrapped: %w", marked)
	if !errors.As(wrapped, &notFound) {
		t.Fatal("wrapped error was not classified as not found")
	}
	if wrapped.Error() != "wrapped: "+cause.Error() {
		t.Errorf("wrapped error = %q, want %q", wrapped.Error(), "wrapped: "+cause.Error())
	}
	if !errors.Is(marked, cause) {
		t.Error("a marked error no longer unwraps to its cause")
	}
}
