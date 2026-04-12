package server

import (
	"encoding/json"
	"net/http"

	"github.com/karthikcodes/aetronyx/internal/config"
)

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	cfg     *config.Config
	version string
}

// NewHandler creates a new Handler.
func NewHandler(cfg *config.Config, version string) *Handler {
	return &Handler{
		cfg:     cfg,
		version: version,
	}
}

// Health handles GET /api/v1/health.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{
		"status":  "ok",
		"version": h.version,
	}
	writeJSON(w, http.StatusOK, resp)
}

// Version handles GET /api/v1/version.
func (h *Handler) Version(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{
		"version":  h.version,
		"commit":   "", // TODO(M1): set from ldflags
		"built_at": "", // TODO(M1): set from ldflags
	}
	writeJSON(w, http.StatusOK, resp)
}

// writeJSON writes a response as JSON with the given status code.
func writeJSON(w http.ResponseWriter, statusCode int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(v)
}

// errorResponse matches the canonical error format from §4.5.
type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// writeError writes a standard error response.
func writeError(w http.ResponseWriter, statusCode int, code, message string) {
	resp := errorResponse{
		Error: errorDetail{
			Code:    code,
			Message: message,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(resp)
}
