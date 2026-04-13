package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/karthikcodes/aetronyx/internal/config"
)

func newWSHandler(t *testing.T, allowRemote bool) (*Handler, *httptest.Server) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{
		Server: config.Server{
			Host:        "127.0.0.1",
			Port:        7777,
			AllowRemote: allowRemote,
		},
	}
	bus := NewEventBus(log)
	handler := NewHandler(cfg, "v0.0.0", "test-token", bus, log)

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/ws", http.HandlerFunc(handler.WebSocket))

	return handler, httptest.NewServer(mux)
}

// TestWebSocketUpgrade verifies that a valid request upgrades successfully and receives a ping.
func TestWebSocketUpgrade(t *testing.T) {
	_, srv := newWSHandler(t, false)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/ws"

	dialer := websocket.Dialer{
		Subprotocols: []string{wsSubprotocol},
	}

	pingReceived := make(chan struct{}, 1)

	conn, resp, err := dialer.Dial(wsURL, http.Header{
		"Origin": []string{"http://127.0.0.1"},
	})
	if err != nil {
		if resp != nil {
			t.Fatalf("dial failed: %v (status %d)", err, resp.StatusCode)
		}
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	conn.SetPingHandler(func(string) error {
		select {
		case pingReceived <- struct{}{}:
		default:
		}
		return conn.WriteMessage(websocket.PongMessage, nil)
	})

	// Read loop to drive ping handler.
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	select {
	case <-pingReceived:
		// success
	case <-time.After(wsPingInterval + 5*time.Second):
		t.Error("did not receive ping within expected interval")
	}
}

// TestWebSocketBadOrigin verifies that a non-loopback origin is rejected when AllowRemote is false.
func TestWebSocketBadOrigin(t *testing.T) {
	_, srv := newWSHandler(t, false)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/ws"

	dialer := websocket.Dialer{
		Subprotocols: []string{wsSubprotocol},
	}

	_, resp, err := dialer.Dial(wsURL, http.Header{
		"Origin": []string{"http://evil.example.com"},
	})

	if err == nil {
		t.Fatal("expected dial to fail for non-loopback origin, but it succeeded")
	}

	if resp == nil {
		t.Fatal("expected an HTTP response on rejection, got nil")
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden, got %d", resp.StatusCode)
	}
}
