package server

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/karthikcodes/aetronyx/internal/config"
)

func newSSEHandler(t *testing.T) (*Handler, *httptest.Server) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{Server: config.Server{Host: "127.0.0.1", Port: 7777}}
	bus := NewEventBus(log)
	handler := NewHandler(cfg, "v0.0.0", "test-token", bus, log)

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/runs/{id}/stream", http.HandlerFunc(handler.StreamRun))

	return handler, httptest.NewServer(mux)
}

// TestSSEHeartbeat verifies a heartbeat frame arrives within twice the interval.
func TestSSEHeartbeat(t *testing.T) {
	_, srv := newSSEHandler(t)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), heartbeatInterval*2+5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/v1/runs/run-1/stream", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %s", ct)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "event: heartbeat" {
			return // success
		}
	}
	t.Error("did not receive heartbeat before timeout")
}

// TestSSEPublishReceive verifies a published event is received as a data frame.
func TestSSEPublishReceive(t *testing.T) {
	handler, srv := newSSEHandler(t)
	defer srv.Close()

	const runID = "run-publish"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/v1/runs/"+runID+"/stream", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Give the handler time to subscribe.
	time.Sleep(50 * time.Millisecond)

	payload := []byte(`{"type":"status","status":"running"}`)
	handler.bus.Publish(runID, payload)

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == string(payload) {
				return // success
			}
			t.Errorf("unexpected data frame: %s", data)
			return
		}
	}
	t.Error("did not receive data frame before timeout")
}

// TestSSEClientDisconnect verifies the subscriber is cleaned up when the client disconnects.
func TestSSEClientDisconnect(t *testing.T) {
	handler, srv := newSSEHandler(t)
	defer srv.Close()

	const runID = "run-disconnect"

	ctx, cancel := context.WithCancel(context.Background())

	req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/v1/runs/"+runID+"/stream", nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	// Give the handler time to subscribe.
	time.Sleep(50 * time.Millisecond)

	if n := handler.bus.subscriberCount(runID); n != 1 {
		t.Errorf("expected 1 subscriber, got %d", n)
	}

	// Cancel the client context to simulate disconnect.
	cancel()
	resp.Body.Close()

	// Wait for the server handler to clean up.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if handler.bus.subscriberCount(runID) == 0 {
			return // success
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("subscriber not cleaned up after disconnect; count=%d", handler.bus.subscriberCount(runID))
}

// TestEventBusOverflow verifies that publishing to a full subscriber channel drops events
// without panicking and without blocking.
func TestEventBusOverflow(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	bus := NewEventBus(log)

	// Subscribe but never drain.
	ch, unsub := bus.Subscribe("run-overflow")
	defer unsub()
	_ = ch

	// Publish one more than the buffer size — must not panic or block.
	payload := []byte(`{"type":"overflow"}`)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i <= subBufferSize; i++ {
			bus.Publish("run-overflow", payload)
		}
	}()

	select {
	case <-done:
		// success: no panic, no block
	case <-time.After(5 * time.Second):
		t.Fatal("Publish blocked or timed out on full channel")
	}
}

// TestEventBusConcurrent verifies concurrent publishes to multiple subscribers are race-free.
// Run with -race to detect data races.
func TestEventBusConcurrent(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	bus := NewEventBus(log)

	const (
		numPublishers  = 10
		numSubscribers = 5
		numEvents      = 50
	)

	var unsubs []func()
	for i := 0; i < numSubscribers; i++ {
		runID := "run-concurrent"
		ch, unsub := bus.Subscribe(runID)
		unsubs = append(unsubs, unsub)
		// drain in background
		go func(c <-chan []byte) {
			for range c {
			}
		}(ch)
	}

	var wg sync.WaitGroup
	for i := 0; i < numPublishers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numEvents; j++ {
				bus.Publish("run-concurrent", []byte(`{}`))
			}
		}()
	}
	wg.Wait()

	for _, u := range unsubs {
		u()
	}
}
