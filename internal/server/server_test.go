package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/karthikcodes/aetronyx/internal/config"
)

func newTestServer(t *testing.T) *httptest.Server {
	cfg := &config.Config{
		Server: config.Server{
			Host: "127.0.0.1",
			Port: 7777,
		},
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	token := "test-token-12345"
	version := "v0.1.0-m1"
	bus := NewEventBus(log)

	handler := NewHandler(cfg, version, token, bus, log)

	mux := http.NewServeMux()

	mux.Handle("GET /api/v1/health", BearerMiddleware(token)(http.HandlerFunc(handler.Health)))
	mux.Handle("GET /api/v1/version", BearerMiddleware(token)(http.HandlerFunc(handler.Version)))
	mux.Handle("/api/v1/", BearerMiddleware(token)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "http.not_found", "Endpoint not found")
	})))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "http.not_found", "Endpoint not found")
	})

	return httptest.NewServer(LoggingMiddleware(log)(RecoveryMiddleware(log)(mux)))
}

func TestHealthEndpoint(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL+"/api/v1/health", nil)
	req.Header.Set("Authorization", "Bearer test-token-12345")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)

	if data["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", data["status"])
	}
	if data["version"] != "v0.1.0-m1" {
		t.Errorf("expected version=v0.1.0-m1, got %v", data["version"])
	}
}

func TestVersionEndpoint(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL+"/api/v1/version", nil)
	req.Header.Set("Authorization", "Bearer test-token-12345")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)

	if _, ok := data["version"]; !ok {
		t.Error("version field missing")
	}
}

func TestUnauthorized(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	// No Authorization header
	resp, err := http.Get(server.URL + "/api/v1/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}

	var errResp errorResponse
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp.Error.Code != "auth.missing" {
		t.Errorf("expected code=auth.missing, got %s", errResp.Error.Code)
	}
}

func TestWrongToken(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL+"/api/v1/health", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}

	var errResp errorResponse
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp.Error.Code != "auth.invalid" {
		t.Errorf("expected code=auth.invalid, got %s", errResp.Error.Code)
	}
}

func TestNotFound(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL+"/api/v1/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer test-token-12345")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}

	var errResp errorResponse
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp.Error.Code != "http.not_found" {
		t.Errorf("expected code=http.not_found, got %s", errResp.Error.Code)
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()

	// Handler that panics
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	var chain http.Handler = panicHandler
	chain = RecoveryMiddleware(log)(chain)

	mux.Handle("/panic", chain)

	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/panic")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}

	var errResp errorResponse
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp.Error.Code != "internal.panic" {
		t.Errorf("expected code=internal.panic, got %s", errResp.Error.Code)
	}
}

func TestLoadOrGenerateToken(t *testing.T) {
	tmpDir := t.TempDir()

	// First call should generate
	token1, err := LoadOrGenerateToken(tmpDir)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if token1 == "" {
		t.Error("token should not be empty")
	}

	// Second call should return the same token
	token2, err := LoadOrGenerateToken(tmpDir)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if token1 != token2 {
		t.Error("tokens should match")
	}

	// Check file permissions
	info, _ := os.Stat(tmpDir + "/auth-token")
	if info.Mode().Perm() != 0o600 {
		t.Errorf("expected mode 0600, got %o", info.Mode().Perm())
	}
}
