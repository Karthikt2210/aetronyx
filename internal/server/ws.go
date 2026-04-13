package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	wsPingInterval = 20 * time.Second
	wsPongTimeout  = 40 * time.Second
	wsSubprotocol  = "aetronyx.v1"
)

type wsMessage struct {
	Type    string `json:"type"`
	RunID   string `json:"run_id,omitempty"`
	Payload string `json:"payload,omitempty"`
}

// WebSocket handles GET /api/v1/ws.
// Upgrades the connection to WebSocket using the aetronyx.v1 subprotocol.
func (h *Handler) WebSocket(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin:  h.checkWSOrigin,
		Subprotocols: []string{wsSubprotocol},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// upgrader writes the HTTP error response on failure
		h.log.Debug("ws upgrade failed", slog.String("error", err.Error()))
		return
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(wsPongTimeout))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongTimeout))
	})

	ticker := time.NewTicker(wsPingInterval)
	defer ticker.Stop()

	readDone := make(chan struct{})

	go h.wsReadLoop(conn, readDone)

	for {
		select {
		case <-readDone:
			return
		case <-ticker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				h.log.Debug("ws ping failed", slog.String("error", err.Error()))
				return
			}
		}
	}
}

// wsReadLoop reads and dispatches incoming WebSocket messages until the connection closes.
func (h *Handler) wsReadLoop(conn *websocket.Conn, done chan struct{}) {
	defer close(done)

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				h.log.Debug("ws read error", slog.String("error", err.Error()))
			}
			return
		}

		var msg wsMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			h.log.Debug("ws unmarshal error", slog.String("error", err.Error()))
			continue
		}

		switch msg.Type {
		case "subscribe.terminal":
			// TODO(M3): register run terminal subscriber
			h.log.Debug("ws subscribe.terminal", slog.String("run_id", msg.RunID))
		case "terminal.input":
			// TODO(M3): forward to pty; stub — ignored for now
			h.log.Debug("ws terminal.input: stub, ignoring")
		default:
			h.log.Debug("ws unknown message type", slog.String("type", msg.Type))
		}
	}
}

// checkWSOrigin validates the WebSocket upgrade Origin header.
// Allows all origins when cfg.Server.AllowRemote is true;
// otherwise only 127.0.0.1, ::1, and localhost are permitted.
func (h *Handler) checkWSOrigin(r *http.Request) bool {
	if h.cfg.Server.AllowRemote {
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	stripped := strings.TrimPrefix(strings.TrimPrefix(origin, "https://"), "http://")
	host := strings.SplitN(stripped, ":", 2)[0]
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}
