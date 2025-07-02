package cmd

import (
	"fmt"
	"sync"
	"time"
)

// pageMu guards Browser, Page and CurrentElement.
// Only one MCP request (or CLI action) may hold it at a time.
var pageMu sync.Mutex

// withPage serialises access to the shared browser state.
// It waits up to 30 s for the lock; afterwards it returns an error so the
// caller can report “page busy”.
func withPage[R any](fn func() (R, error)) (R, error) {
	const timeout = 30 * time.Second
	var zero R

	locked := make(chan struct{})
	go func() {
		pageMu.Lock()
		close(locked)
	}()

	select {
	case <-locked:
		defer pageMu.Unlock()
		return fn()
	case <-time.After(timeout):
		return zero, fmt.Errorf("page busy: could not acquire lock within %s", timeout)
	}
}
