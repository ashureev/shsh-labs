// Package identity provides anonymous per-device identity primitives.
package identity

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/ashureev/shsh-labs/internal/domain"
	"github.com/ashureev/shsh-labs/internal/store"
)

const (
	AnonCookieName        = "shsh_anon_id"
	SessionHeaderName     = "X-SHSH-Session-ID"
	DefaultSessionIDValue = "default"
	anonCookieMaxAge      = 30 * 24 * time.Hour
)

type contextKey int

const (
	userIDKey contextKey = iota
	usernameKey
	sessionIDKey
)

var (
	anonIDPattern    = regexp.MustCompile(`^anon_[a-f0-9]{32}$`)
	sessionIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)
)

// UserIDFromContext extracts the user ID from the request context.
func UserIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(userIDKey).(string); ok {
		return v
	}
	return ""
}

// UsernameFromContext extracts the username from the request context.
func UsernameFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(usernameKey).(string); ok {
		return v
	}
	return ""
}

// SessionIDFromContext extracts the tab session ID from the request context.
func SessionIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(sessionIDKey).(string); ok {
		return v
	}
	return DefaultSessionIDValue
}

func generateAnonID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate anonymous id: %w", err)
	}
	return "anon_" + hex.EncodeToString(buf), nil
}

func isValidAnonID(id string) bool {
	return anonIDPattern.MatchString(id)
}

func sanitizeSessionID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" || !sessionIDPattern.MatchString(id) {
		return DefaultSessionIDValue
	}
	return id
}

func deriveUsername(userID string) string {
	if len(userID) > 13 {
		return "anon-" + userID[len(userID)-8:]
	}
	return "anon-user"
}

func ensureUser(ctx context.Context, repo store.Repository, userID string) error {
	user, err := repo.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	if user != nil {
		return nil
	}

	now := time.Now()
	return repo.UpsertUser(ctx, &domain.User{
		UserID:     userID,
		Username:   deriveUsername(userID),
		VolumePath: "playground-" + userID + "-data",
		LastSeenAt: now,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
}

func getOrCreateAnonID(w http.ResponseWriter, r *http.Request, isDev bool) (string, error) {
	if c, err := r.Cookie(AnonCookieName); err == nil && isValidAnonID(c.Value) {
		http.SetCookie(w, &http.Cookie{
			Name:     AnonCookieName,
			Value:    c.Value,
			Path:     "/",
			MaxAge:   int(anonCookieMaxAge.Seconds()),
			Expires:  time.Now().Add(anonCookieMaxAge),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   !isDev,
		})
		return c.Value, nil
	}

	id, err := generateAnonID()
	if err != nil {
		return "", err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     AnonCookieName,
		Value:    id,
		Path:     "/",
		MaxAge:   int(anonCookieMaxAge.Seconds()),
		Expires:  time.Now().Add(anonCookieMaxAge),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   !isDev,
	})
	return id, nil
}

func sessionIDFromRequest(r *http.Request) string {
	sid := r.Header.Get(SessionHeaderName)
	if sid == "" {
		sid = r.URL.Query().Get("session_id")
	}
	return sanitizeSessionID(sid)
}

// Middleware injects anonymous per-device identity and per-request session ID.
func Middleware(repo store.Repository, isDev bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, err := getOrCreateAnonID(w, r, isDev)
			if err != nil {
				http.Error(w, `{"error":"failed to establish anonymous identity"}`, http.StatusInternalServerError)
				return
			}

			if err := ensureUser(r.Context(), repo, userID); err != nil {
				http.Error(w, `{"error":"failed to initialize anonymous user"}`, http.StatusInternalServerError)
				return
			}

			username := deriveUsername(userID)
			sessionID := sessionIDFromRequest(r)

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			ctx = context.WithValue(ctx, usernameKey, username)
			ctx = context.WithValue(ctx, sessionIDKey, sessionID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// IPFromRequest returns a normalized remote IP for optional request tracing.
func IPFromRequest(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
