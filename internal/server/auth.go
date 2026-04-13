package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// LoadOrGenerateToken reads the auth token from disk or generates a new one.
// Token is stored at dataDir/auth-token with mode 0600.
func LoadOrGenerateToken(dataDir string) (string, error) {
	tokenPath := filepath.Join(dataDir, "auth-token")

	// Try to read existing token
	if data, err := os.ReadFile(tokenPath); err == nil {
		return strings.TrimSpace(string(data)), nil
	}

	// Generate 32 random bytes and hex-encode
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}

	token := hex.EncodeToString(buf)

	// Write with mode 0600
	if err := os.WriteFile(tokenPath, []byte(token), 0o600); err != nil {
		return "", fmt.Errorf("failed to write token: %w", err)
	}

	return token, nil
}

// BearerMiddleware returns a middleware that validates the Authorization header
// or the aetronyx_token cookie (fallback for SSE clients that cannot set headers).
// Returns 401 with a standard error format if no valid credential is found.
func BearerMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bearerAuth := r.Header.Get("Authorization")

			// Check Authorization: Bearer <token>
			const bearerPrefix = "Bearer "
			if bearerAuth != "" && strings.HasPrefix(bearerAuth, bearerPrefix) {
				provided := bearerAuth[len(bearerPrefix):]
				if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) == 1 {
					next.ServeHTTP(w, r)
					return
				}
				// Header present but wrong — skip cookie check, reject immediately.
				writeError(w, http.StatusUnauthorized, "auth.invalid", "Invalid token")
				return
			}

			// Cookie fallback (used by EventSource which cannot set headers).
			if cookie, err := r.Cookie("aetronyx_token"); err == nil {
				if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(token)) == 1 {
					next.ServeHTTP(w, r)
					return
				}
			}

			writeError(w, http.StatusUnauthorized, "auth.missing", "Authorization header required")
		})
	}
}

// AuthHandshake handles POST /api/v1/auth/handshake.
// Validates the token in the request body and sets the aetronyx_token httpOnly cookie on success.
func (h *Handler) AuthHandshake(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "auth.bad_request", "Invalid request body")
		return
	}
	if subtle.ConstantTimeCompare([]byte(body.Token), []byte(h.authToken)) != 1 {
		writeError(w, http.StatusUnauthorized, "auth.invalid", "Invalid token")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "aetronyx_token",
		Value:    body.Token,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	})
	w.WriteHeader(http.StatusNoContent)
}

// AuthLogout handles POST /api/v1/auth/logout.
// Clears the aetronyx_token cookie and returns 204.
func (h *Handler) AuthLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "aetronyx_token",
		Value:    "",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}
