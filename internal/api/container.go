package api

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/ashureev/shsh-labs/internal/config"
	"github.com/ashureev/shsh-labs/internal/identity"
	"github.com/ashureev/shsh-labs/internal/shared"
	"github.com/ashureev/shsh-labs/internal/store"
	"github.com/go-chi/chi/v5"
)

// provisionLocks prevents concurrent provisioning for the same user.
var provisionLocks sync.Map

// destroyLocks prevents concurrent destroy requests for the same user.
var destroyLocks sync.Map

// ContainerHandler handles container-related endpoints.
type ContainerHandler struct {
	*Handler
	aiEnabled bool
	cfg       *config.Config
}

// NewContainerHandlerWithAI creates a new container handler with AI enabled flag.
// Deprecated: Use NewContainerHandlerWithAIAndConfig instead.
func NewContainerHandlerWithAI(base *Handler, aiEnabled bool) *ContainerHandler {
	return &ContainerHandler{Handler: base, aiEnabled: aiEnabled}
}

// NewContainerHandler creates a new container handler (AI disabled by default).
// Deprecated: Use NewContainerHandlerWithConfig instead.
func NewContainerHandler(base *Handler) *ContainerHandler {
	return &ContainerHandler{Handler: base, aiEnabled: false}
}

// NewContainerHandlerWithAIAndConfig creates a new container handler with AI enabled flag and config.
func NewContainerHandlerWithAIAndConfig(base *Handler, aiEnabled bool, cfg *config.Config) *ContainerHandler {
	return &ContainerHandler{Handler: base, aiEnabled: aiEnabled, cfg: cfg}
}

// NewContainerHandlerWithConfig creates a new container handler with config (AI disabled by default).
func NewContainerHandlerWithConfig(base *Handler, cfg *config.Config) *ContainerHandler {
	return &ContainerHandler{Handler: base, aiEnabled: false, cfg: cfg}
}

// RegisterRoutes registers container routes.
func (h *ContainerHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api", func(r chi.Router) {
		r.Get("/me", h.GetMe)
		r.Get("/config", h.GetConfig)
		r.Post("/provision", h.Provision)
		r.Post("/destroy", h.Destroy)
	})
}

// GetMe returns the current user's information.
func (h *ContainerHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID := identity.UserIDFromContext(r.Context())
	if userID == "" {
		Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.repo.GetUser(r.Context(), userID)
	if err != nil || user == nil {
		Error(w, http.StatusUnauthorized, "user not found")
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"user_id":       user.UserID,
		"username":      user.Username,
		"container_id":  user.ContainerID,
		"container_ttl": int64(user.SessionTTL(60 * time.Minute).Seconds()),
	})
}

// GetConfig returns the server configuration for the frontend.
func (h *ContainerHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, map[string]interface{}{
		"ai_enabled": h.aiEnabled,
	})
}

// Provision creates and starts a container for the user.
func (h *ContainerHandler) Provision(w http.ResponseWriter, r *http.Request) {
	userID := identity.UserIDFromContext(r.Context())

	// Prevent concurrent provisioning requests.
	lock, _ := provisionLocks.LoadOrStore(userID, &sync.Mutex{})
	mutex := lock.(*sync.Mutex)
	if !mutex.TryLock() {
		slog.Warn("Provisioning already in progress", "user_id", userID)
		Error(w, http.StatusConflict, "provisioning_in_progress")
		return
	}
	defer func() {
		mutex.Unlock()
		provisionLocks.Delete(userID)
	}()

	ctx := r.Context()
	user, err := h.repo.GetUser(ctx, userID)
	if err != nil || user == nil {
		slog.Error("Failed to get user for provisioning", "error", err, "user_id", userID)
		Error(w, http.StatusUnauthorized, "user not found")
		return
	}

	slog.Info("Provisioning container", "user_id", userID, "volume_path", user.VolumePath)

	containerID, err := h.mgr.EnsureContainer(ctx, userID, user.ContainerID, user.LastSeenAt, nil)
	if err != nil {
		slog.Error("Failed to provision container", "error", err, "user_id", userID)
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := h.repo.UpdateContainerID(ctx, userID, containerID, ""); err != nil {
		slog.Error("Failed to update container ID", "error", err, "user_id", userID)
		Error(w, http.StatusInternalServerError, "failed to update container state")
		return
	}

	slog.Info("Container provisioned", "user_id", userID, "container_id", containerID)
	JSON(w, http.StatusOK, map[string]interface{}{
		"status":       "ready",
		"container_id": containerID,
	})
}

// Destroy stops and removes the user's container.
func (h *ContainerHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	userID := identity.UserIDFromContext(r.Context())
	ctx := r.Context()

	// Prevent concurrent destroy requests.
	lock, _ := destroyLocks.LoadOrStore(userID, &sync.Mutex{})
	mutex := lock.(*sync.Mutex)
	if !mutex.TryLock() {
		slog.Warn("Destroy already in progress", "user_id", userID)
		JSON(w, http.StatusOK, map[string]string{"status": "destroying"})
		return
	}
	defer func() {
		mutex.Unlock()
		destroyLocks.Delete(userID)
	}()

	user, err := h.repo.GetUser(ctx, userID)
	if err != nil || user == nil {
		slog.Error("Failed to get user for destruction", "error", err, "user_id", userID)
		Error(w, http.StatusUnauthorized, "user not found")
		return
	}

	// Close any active terminal session.
	h.sm.CloseSession(userID)

	// Python Agent handles session cleanup via Redis TTL

	if user.ContainerID != "" {
		slog.Info("Destroying container", "user_id", userID, "container_id", user.ContainerID)

		// Clear container binding immediately so terminate is instant from user perspective.
		if err := h.updateContainerIDWithRetry(ctx, userID, "", user.ContainerID); err != nil {
			slog.Error("Failed to clear container ID", "error", err, "user_id", userID)
			Error(w, http.StatusInternalServerError, "failed to update database state")
			return
		}

		containerID := user.ContainerID
		// Use config timeout if available
		destroyTimeout := 30 * time.Second
		if h.cfg != nil {
			destroyTimeout = h.cfg.Timeout.DestroyCleanup
		}
		go func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), destroyTimeout)
			defer cancel()

			if err := h.mgr.StopContainer(cleanupCtx, containerID); err != nil {
				slog.Error("Failed to stop container", "error", err, "container_id", containerID, "user_id", userID)
			} else {
				slog.Info("Container stop/remove completed", "container_id", containerID, "user_id", userID)
			}
		}()
	}

	slog.Info("Container destroyed", "user_id", userID)
	JSON(w, http.StatusOK, map[string]string{"status": "destroyed"})
}

// updateContainerIDWithRetry attempts to update container ID with exponential backoff
// to handle SQLITE_BUSY errors during concurrent operations.
func (h *ContainerHandler) updateContainerIDWithRetry(ctx context.Context, userID, newID, expectedID string) error {
	maxRetries := 3
	baseDelay := 50 * time.Millisecond

	if h.cfg != nil {
		maxRetries = h.cfg.Retry.DatabaseMaxRetries
		baseDelay = h.cfg.Retry.DatabaseRetryBaseDelay
	}

	for i := 0; i < maxRetries; i++ {
		err := h.repo.UpdateContainerID(ctx, userID, newID, expectedID)
		if err == nil {
			return nil
		}

		// Check if it's a SQLITE_BUSY or locked error
		if shared.IsSQLiteConflictError(err) {
			if i < maxRetries-1 {
				delay := baseDelay * time.Duration(1<<i) // exponential backoff
				slog.Debug("Database locked during container ID update, retrying",
					"user_id", userID,
					"attempt", i+1,
					"delay", delay)
				time.Sleep(delay)
				continue
			}
		}

		// Check for context canceled - this is not fatal for cleanup
		if ctx.Err() != nil {
			slog.Debug("Context canceled during container ID update, cleanup may be incomplete",
				"user_id", userID,
				"error", err)
			return nil
		}

		// Non-retryable error or max retries exceeded
		return err
	}

	return nil
}

// HealthHandler handles health check endpoints.
type HealthHandler struct {
	repo store.Repository
	cfg  *config.Config
}

// NewHealthHandler creates a new health handler.
// Deprecated: Use NewHealthHandlerWithConfig instead.
func NewHealthHandler(repo store.Repository) *HealthHandler {
	return &HealthHandler{repo: repo}
}

// NewHealthHandlerWithConfig creates a new health handler with configuration.
func NewHealthHandlerWithConfig(repo store.Repository, cfg *config.Config) *HealthHandler {
	return &HealthHandler{repo: repo, cfg: cfg}
}

// Health returns the health status of the API and its dependencies.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	healthCheckTimeout := 5 * time.Second
	if h.cfg != nil {
		healthCheckTimeout = h.cfg.Timeout.HealthCheck
	}
	ctx, cancel := context.WithTimeout(r.Context(), healthCheckTimeout)
	defer cancel()

	status := map[string]interface{}{
		"status": "healthy",
		"checks": map[string]string{"api": "ok"},
	}
	statusCode := http.StatusOK

	if err := h.repo.Ping(ctx); err != nil {
		slog.Error("Health check failed", "error", err)
		status["status"] = "degraded"
		status["checks"].(map[string]string)["database"] = "unreachable"
		statusCode = http.StatusServiceUnavailable
	} else {
		status["checks"].(map[string]string)["database"] = "ok"
	}

	JSON(w, statusCode, status)
}

// RegisterHealth registers the health check route.
func (h *HealthHandler) RegisterHealth(r chi.Router) {
	r.Get("/health", h.Health)
}
