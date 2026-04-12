package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
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

// BearerMiddleware returns a middleware that validates the Authorization header.
// Returns 401 with standard error format if token is missing or incorrect.
func BearerMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")

			// Extract bearer token
			const bearerPrefix = "Bearer "
			if len(auth) < len(bearerPrefix) || !strings.HasPrefix(auth, bearerPrefix) {
				writeError(w, http.StatusUnauthorized, "auth.missing", "Authorization header required")
				return
			}

			provided := auth[len(bearerPrefix):]

			// Use constant-time comparison to prevent timing attacks
			if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
				writeError(w, http.StatusUnauthorized, "auth.invalid", "Invalid token")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
