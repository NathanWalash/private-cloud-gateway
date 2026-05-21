package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

// LogsStream streams container logs as Server-Sent Events.
// GET /api/apps/:id/logs/stream?tail=50
func (h *Handler) LogsStream(w http.ResponseWriter, r *http.Request) {
	if h.docker == nil {
		http.Error(w, "Docker unavailable", http.StatusServiceUnavailable)
		return
	}
	id, err := pathID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var containerName string
	if err := h.db.QueryRowContext(r.Context(),
		"SELECT container_name FROM apps WHERE id = ?", id).Scan(&containerName); err != nil {
		http.Error(w, "app not found", http.StatusNotFound)
		return
	}

	tail := 50
	if t := r.URL.Query().Get("tail"); t != "" {
		if n, err := strconv.Atoi(t); err == nil && n > 0 && n <= 500 {
			tail = n
		}
	}

	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering if present

	flusher, canFlush := w.(http.Flusher)

	// Send historical logs first
	hist, err := h.docker.Logs(r.Context(), containerName, tail)
	if err == nil && hist != "" {
		for _, line := range strings.Split(hist, "\n") {
			if line == "" {
				continue
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", sseEscape(line))
		}
		if canFlush {
			flusher.Flush()
		}
	}

	// Stream live logs
	rc, err := h.docker.LogsFollow(r.Context(), containerName)
	if err != nil {
		_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", sseEscape(err.Error()))
		if canFlush {
			flusher.Flush()
		}
		slog.Warn("logs stream failed", "container", containerName, "err", err)
		return
	}
	defer rc.Close()

	buf := make([]byte, 4096)
	hdr := make([]byte, 8)
	for {
		select {
		case <-r.Context().Done():
			return
		default:
		}

		// Read Docker mux header
		if _, err := readFull(rc, hdr); err != nil {
			return
		}
		size := int(hdr[4])<<24 | int(hdr[5])<<16 | int(hdr[6])<<8 | int(hdr[7])
		if size <= 0 {
			continue
		}

		// Read the log chunk
		for size > 0 {
			n := size
			if n > len(buf) {
				n = len(buf)
			}
			nr, err := rc.Read(buf[:n])
			if nr > 0 {
				line := strings.TrimRight(string(buf[:nr]), "\n\r")
				if line != "" {
					_, _ = fmt.Fprintf(w, "data: %s\n\n", sseEscape(line))
					if canFlush {
						flusher.Flush()
					}
				}
				size -= nr
			}
			if err != nil {
				return
			}
		}
	}
}

func sseEscape(s string) string {
	return strings.ReplaceAll(s, "\n", "\\n")
}

// readFull reads exactly len(buf) bytes.
func readFull(r interface{ Read([]byte) (int, error) }, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
