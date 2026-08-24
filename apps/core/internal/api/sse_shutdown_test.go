package api

import (
	"net/http/httptest"
	"testing"
	"time"
)

// TestAppEvents_ReturnsOnShutdown verifies the SSE handler unblocks promptly when
// the process begins shutting down, so graceful shutdown isn't held up for its
// full drain timeout. NOTE: Shutdown() closes a process-wide channel, so this is
// the only streaming test that may run (kept isolated deliberately).
func TestAppEvents_ReturnsOnShutdown(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/apps/events", nil)

	done := make(chan struct{})
	go func() {
		(&Handler{}).AppEvents(rec, req)
		close(done)
	}()

	// Let the handler subscribe and enter its select loop.
	time.Sleep(30 * time.Millisecond)
	Shutdown()

	select {
	case <-done:
		// Returned promptly — correct.
	case <-time.After(2 * time.Second):
		t.Fatal("AppEvents did not return after Shutdown()")
	}
}
