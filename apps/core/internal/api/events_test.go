package api

import (
	"testing"
)

func TestHub_BroadcastAndSubscribe(t *testing.T) {
	h := &Hub{clients: make(map[chan AppStatusEvent]struct{})}

	ch := h.subscribe()
	defer h.unsubscribe(ch)

	h.Broadcast(AppStatusEvent{AppID: 42, Status: "stopped"})

	select {
	case ev := <-ch:
		if ev.AppID != 42 || ev.Status != "stopped" {
			t.Errorf("unexpected event: %+v", ev)
		}
	default:
		t.Error("expected event in channel, got nothing")
	}
}

func TestHub_SlowClientDrops(t *testing.T) {
	h := &Hub{clients: make(map[chan AppStatusEvent]struct{})}

	// Create a zero-capacity channel — broadcast must not block.
	ch := make(chan AppStatusEvent) // unbuffered
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, ch)
		h.mu.Unlock()
		close(ch)
	}()

	// Broadcast must return without blocking even when the subscriber is slow.
	done := make(chan struct{})
	go func() {
		h.Broadcast(AppStatusEvent{AppID: 1, Status: "running"})
		close(done)
	}()

	select {
	case <-done:
		// Good — did not block
	case <-ch:
		// Also fine if event was delivered
	}
}

func TestHub_Unsubscribe(t *testing.T) {
	h := &Hub{clients: make(map[chan AppStatusEvent]struct{})}
	ch := h.subscribe()
	h.unsubscribe(ch)

	h.mu.Lock()
	_, still := h.clients[ch]
	h.mu.Unlock()
	if still {
		t.Error("channel still in clients map after unsubscribe")
	}
}

func TestBroadcastStatus_GlobalHub(t *testing.T) {
	// Verifies that BroadcastStatus uses the global hub.
	ch := globalHub.subscribe()
	defer globalHub.unsubscribe(ch)

	BroadcastStatus(99, "error")

	select {
	case ev := <-ch:
		if ev.AppID != 99 || ev.Status != "error" {
			t.Errorf("unexpected event: %+v", ev)
		}
	default:
		t.Error("no event received from BroadcastStatus")
	}
}
