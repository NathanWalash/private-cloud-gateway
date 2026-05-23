package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// AppStatusEvent is pushed over SSE whenever an app's status changes.
type AppStatusEvent struct {
	AppID  int64  `json:"app_id"`
	Status string `json:"status"`
}

// Hub broadcasts app status changes to all connected SSE clients.
type Hub struct {
	mu      sync.Mutex
	clients map[chan AppStatusEvent]struct{}
}

var globalHub = &Hub{clients: make(map[chan AppStatusEvent]struct{})}

// Broadcast sends an event to all connected clients (non-blocking).
func (h *Hub) Broadcast(ev AppStatusEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- ev:
		default:
			// Slow client — drop the event rather than block
		}
	}
}

func (h *Hub) subscribe() chan AppStatusEvent {
	ch := make(chan AppStatusEvent, 16)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *Hub) unsubscribe(ch chan AppStatusEvent) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
	close(ch)
}

// BroadcastStatus pushes an app status change to all SSE subscribers.
// Called from the health polling goroutine in main.go.
func BroadcastStatus(appID int64, status string) {
	globalHub.Broadcast(AppStatusEvent{AppID: appID, Status: status})
}

// AppEvents streams real-time app status changes via SSE.
// GET /api/apps/events
func (h *Handler) AppEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	ch := globalHub.subscribe()
	defer globalHub.unsubscribe(ch)

	// Send keep-alive comment
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(ev)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
