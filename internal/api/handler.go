// Package api provides HTTP handlers for the SHSH API.
package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/ashureev/shsh-labs/internal/container"
	"github.com/ashureev/shsh-labs/internal/store"
	"github.com/ashureev/shsh-labs/internal/terminal"
)

// Handler provides common handler utilities.
type Handler struct {
	repo                store.Repository
	mgr                 container.Manager
	sm                  *terminal.SessionManager
	frontendRedirectURL string
}

// NewHandler creates a new Handler with common dependencies.
func NewHandler(repo store.Repository, mgr container.Manager, sm *terminal.SessionManager, frontendURL string) *Handler {
	return &Handler{
		repo:                repo,
		mgr:                 mgr,
		sm:                  sm,
		frontendRedirectURL: frontendURL,
	}
}

// JSON writes a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, `{"error": "failed to encode response"}`, http.StatusInternalServerError)
	}
}

// Error writes a JSON error response.
func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, map[string]string{"error": message})
}

// isDevelopment returns true if running in development mode.
func (h *Handler) isDevelopment() bool {
	if env := os.Getenv("APP_ENV"); env != "" {
		return env == "development"
	}
	// Fallback to URL detection for now
	return h.frontendRedirectURL == "" ||
		h.frontendRedirectURL == "/dashboard" ||
		strings.Contains(h.frontendRedirectURL, "localhost") ||
		strings.Contains(h.frontendRedirectURL, "127.0.0.1")
}
