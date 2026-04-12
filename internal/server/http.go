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
	handler := NewHandler(cfg, version)

	// Create mux with routes
	mux := http.NewServeMux()

	// Middleware chain: recovery → logging → bearer auth
	var apiChain http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/health":
			handler.Health(w, r)
		case "/api/v1/version":
			handler.Version(w, r)
		default:
			writeError(w, http.StatusNotFound, "http.not_found", "Endpoint not found")
		}
	})

	apiChain = BearerMiddleware(token)(apiChain)
	apiChain = LoggingMiddleware(log)(apiChain)
	apiChain = RecoveryMiddleware(log)(apiChain)

	mux.Handle("/api/v1/", apiChain)

	// 404 for non-API paths
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "http.not_found", "Endpoint not found")
	})

	httpSrv := &http.Server{
		Addr:           net.JoinHostPort(cfg.Server.Host, fmt.Sprintf("%d", cfg.Server.Port)),
		Handler:        mux,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
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

	// Check if binding to non-loopback
	if !isLoopback(host) {
		s.log.Warn("server listening on non-loopback address (insecure for untrusted networks)",
			slog.String("address", host),
		)
	}

	s.log.Info("server starting",
		slog.String("address", host),
	)

	// ListenAndServe blocks until an error occurs
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
