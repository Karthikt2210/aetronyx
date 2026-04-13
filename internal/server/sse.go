package server

import (
	"fmt"
	"net/http"
	"time"
)

const heartbeatInterval = 15 * time.Second

// StreamRun handles GET /api/v1/runs/{id}/stream.
// Streams SSE events for the given run until the client disconnects.
func (h *Handler) StreamRun(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	if runID == "" {
		writeError(w, http.StatusBadRequest, "run.id_required", "run ID is required")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		panic("SSE: ResponseWriter does not implement http.Flusher")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch, unsub := h.bus.Subscribe(runID)
	defer unsub()

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			fmt.Fprintf(w, "event: heartbeat\n\n")
			flusher.Flush()
		case payload, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		}
	}
}
