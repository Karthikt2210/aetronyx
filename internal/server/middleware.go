package server

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"sync"
	"time"
)

// statusWriter wraps http.ResponseWriter to capture status code.
type statusWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// LoggingMiddleware logs requests with method, path, status code, and duration.
func LoggingMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := &statusWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)
			log.InfoContext(r.Context(), "request completed",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", wrapped.statusCode),
				slog.Duration("duration", duration),
			)
		})
	}
}

// RecoveryMiddleware recovers from panics, logs the stack trace, and returns 500.
func RecoveryMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Use sync.Once to ensure we only write the error response once
			var once sync.Once
			defer func() {
				if v := recover(); v != nil {
					once.Do(func() {
						stack := string(debug.Stack())
						log.ErrorContext(r.Context(), "panic recovered",
							slog.Any("panic", v),
							slog.String("stack", stack),
						)
						writeError(w, http.StatusInternalServerError, "internal.panic", "Internal server error")
					})
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
