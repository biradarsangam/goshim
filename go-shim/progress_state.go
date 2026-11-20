// progress_state.go maintains byte-level progress snapshots for active
// pulls and pushes. Call Progress with the same reference passed to Pull
// or Push to read its current status and byte counts.
package goshim

import (
	"sync"
)

type progressEntry struct {
	status    string
	total     int64
	completed int64
}

var progressState struct {
	mu      sync.Mutex
	entries map[string]*progressEntry
}

// entryLocked returns key's entry, creating it if absent. Callers must
// hold progressState.mu. An empty key means "don't track".
func entryLocked(key string) *progressEntry {
	if progressState.entries == nil {
		progressState.entries = make(map[string]*progressEntry)
	}
	e, ok := progressState.entries[key]
	if !ok {
		e = &progressEntry{}
		progressState.entries[key] = e
	}
	return e
}

// progressReset clears any leftover total/completed from a previous
// pull/push of the same key and sets the initial status text.
func progressReset(key, status string) {
	if key == "" {
		return
	}
	progressState.mu.Lock()
	defer progressState.mu.Unlock()
	if progressState.entries == nil {
		progressState.entries = make(map[string]*progressEntry)
	}
	progressState.entries[key] = &progressEntry{status: status}
}

// progressSetStatus updates only the status text (e.g. "pulling manifest"
// -> "pulling"), leaving the running totals untouched.
func progressSetStatus(key, status string) {
	if key == "" {
		return
	}
	progressState.mu.Lock()
	defer progressState.mu.Unlock()
	entryLocked(key).status = status
}

// progressAddTotal adjusts key's running total by delta bytes — positive
// when a new layer/blob's size becomes known and it's about to be
// downloaded/uploaded, negative to undo that once a blob turns out to
// already exist at the destination and no bytes will actually move for it
// (see backend_podman.go's ProgressEventSkipped handling).
func progressAddTotal(key string, delta int64) {
	if key == "" || delta == 0 {
		return
	}
	progressState.mu.Lock()
	defer progressState.mu.Unlock()
	entryLocked(key).total += delta
}

// progressAddCompleted adds delta bytes to key's running completed count.
// Called from the same places that already increment an mpb bar (see
// proxyOrNop in shared_oci.go), so the two stay in lockstep.
func progressAddCompleted(key string, delta int64) {
	if key == "" || delta <= 0 {
		return
	}
	progressState.mu.Lock()
	defer progressState.mu.Unlock()
	entryLocked(key).completed += delta
}

// progressDone removes key's entry once its pull/push has finished.
// Safe to call for a key that was never tracked.
func progressDone(key string) {
	if key == "" {
		return
	}
	progressState.mu.Lock()
	defer progressState.mu.Unlock()
	delete(progressState.entries, key)
}
