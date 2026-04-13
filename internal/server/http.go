package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/karthikcodes/aetronyx/internal/config"
)

// Server wraps the HTTP server with graceful shutdown.
type Server struct {
	httpSrv *http.Server
	log     *slog.Logger
}

// New creates and configures a new HTTP server.
func New(cfg *config.Config, token, version string, log *slog.Logger) *Server {
	bus := NewEventBus(log)
	handler := NewHandler(cfg, version, token, bus, log)

	mux := http.NewServeMux()

	// protect wraps a handler with bearer/cookie auth + logging + recovery.
	protect := func(h http.HandlerFunc) http.Handler {
		var chain http.Handler = http.HandlerFunc(h)
		chain = BearerMiddleware(token)(chain)
		chain = LoggingMiddleware(log)(chain)
		chain = RecoveryMiddleware(log)(chain)
		return chain
	}

	// logged wraps a handler with logging + recovery only (no auth).
	logged := func(h http.HandlerFunc) http.Handler {
		var chain http.Handler = http.HandlerFunc(h)
		chain = LoggingMiddleware(log)(chain)
		chain = RecoveryMiddleware(log)(chain)
		return chain
	}

	// Unauthenticated auth endpoints.
	mux.Handle("POST /api/v1/auth/handshake", logged(handler.AuthHandshake))
	mux.Handle("POST /api/v1/auth/logout", logged(handler.AuthLogout))

	// Protected endpoints.
	mux.Handle("GET /api/v1/health", protect(handler.Health))
	mux.Handle("GET /api/v1/version", protect(handler.Version))
	mux.Handle("GET /api/v1/runs/{id}/stream", protect(handler.StreamRun))
	mux.Handle("GET /api/v1/ws", protect(handler.WebSocket))

	// SPA fallback.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "http.not_found", "Endpoint not found")
	})

	httpSrv := &http.Server{
		Addr:    net.JoinHostPort(cfg.Server.Host, fmt.Sprintf("%d", cfg.Server.Port)),
		Handler: mux,
		ReadTimeout: 30 * time.Second,
		// WriteTimeout is intentionally unset (0) to support SSE streaming responses.
		// TODO(M4): introduce per-handler timeouts via http.TimeoutHandler for non-streaming routes.
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	return &Server{
		httpSrv: httpSrv,
		log:     log,
	}
}

// Start begins listening on the configured address.
// Logs a warning to stderr if binding to non-loopback without --allow-remote.
func (s *Server) Start() error {
	cfg := s.httpSrv
	host := cfg.Addr

	if !isLoopback(host) {
		s.log.Warn("server listening on non-loopback address (insecure for untrusted networks)",
			slog.String("address", host),
		)
	}

	s.log.Info("server starting",
		slog.String("address", host),
	)

	if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the server with a 10-second timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
	}

	return s.httpSrv.Shutdown(ctx)
}

// isLoopback checks if a host:port is on loopback (127.0.0.1 or ::1).
func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}
