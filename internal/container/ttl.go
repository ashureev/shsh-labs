package container

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ashureev/shsh-labs/internal/store"
)

// deleteAgentSessionWithRetry attempts to delete an agent session with
// exponential backoff to handle SQLITE_BUSY errors.
func deleteAgentSessionWithRetry(ctx context.Context, repo store.Repository, userID string) error {
	maxRetries := 3
	baseDelay := 100 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		err := repo.DeleteAgentSession(ctx, userID)
		if err == nil {
			return nil
		}

		// Check if it's a SQLITE_BUSY error
		if strings.Contains(err.Error(), "database is locked") || strings.Contains(err.Error(), "SQLITE_BUSY") {
			if i < maxRetries-1 {
				delay := baseDelay * time.Duration(1<<i) // exponential backoff: 100ms, 200ms, 400ms
				slog.Debug("Agent session delete failed with SQLITE_BUSY, retrying",
					"user_id", userID,
					"attempt", i+1,
					"delay", delay)
				time.Sleep(delay)
				continue
			}
		}

		// Non-retryable error or max retries exceeded
		return fmt.Errorf("failed to delete agent session for %s after %d attempts: %w", userID, maxRetries, err)
	}

	return nil
}

// updateContainerIDWithRetry attempts to update container ID with exponential backoff
// to handle SQLITE_BUSY errors.
func updateContainerIDWithRetry(ctx context.Context, repo store.Repository, userID, newID, expectedID string) error {
	maxRetries := 3
	baseDelay := 50 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		err := repo.UpdateContainerID(ctx, userID, newID, expectedID)
		if err == nil {
			return nil
		}

		// Check if it's a SQLITE_BUSY or locked error
		errStr := err.Error()
		if strings.Contains(errStr, "database is locked") || strings.Contains(errStr, "SQLITE_BUSY") {
			if i < maxRetries-1 {
				delay := baseDelay * time.Duration(1<<i) // exponential backoff: 50ms, 100ms, 200ms
				slog.Debug("TTL worker: Database locked during container ID update, retrying",
					"user_id", userID,
					"attempt", i+1,
					"delay", delay)
				time.Sleep(delay)
				continue
			}
		}

		// Check for context canceled - this is not fatal for cleanup
		if ctx.Err() != nil {
			slog.Debug("TTL worker: Context canceled during container ID update, cleanup may be incomplete",
				"user_id", userID,
				"error", err)
			return nil
		}

		// Non-retryable error or max retries exceeded
		return err
	}

	return nil
}

const ttlWorkerInterval = 5 * time.Minute

// CleanupCallback is called when a session is cleaned up by the TTL worker.
type CleanupCallback func(userID string)

// StartTTLWorker runs a background goroutine that periodically sweeps for
// inactive sessions and cleans up associated containers.
func StartTTLWorker(ctx context.Context, repo store.Repository, mgr Manager, ttl time.Duration, onCleanup CleanupCallback) {
	ticker := time.NewTicker(ttlWorkerInterval)
	go func() {
		defer ticker.Stop()
		slog.Info("TTL worker started", "interval", ttlWorkerInterval, "ttl", ttl)

		for {
			select {
			case <-ticker.C:
				cleanupExpiredSessions(ctx, repo, mgr, ttl, onCleanup)
			case <-ctx.Done():
				slog.Info("TTL worker shutting down", "reason", ctx.Err())
				return
			}
		}
	}()
}

func cleanupExpiredSessions(ctx context.Context, repo store.Repository, mgr Manager, ttl time.Duration, onCleanup CleanupCallback) {
	expiredUsers, err := repo.GetExpiredSessions(ctx, ttl)
	if err != nil {
		slog.Error("TTL worker failed to get expired sessions", "error", err)
		return
	}

	if len(expiredUsers) == 0 {
		return
	}

	slog.Info("TTL worker found expired sessions", "count", len(expiredUsers))

	for _, user := range expiredUsers {
		slog.Info("TTL worker cleaning up container",
			"container_id", user.ContainerID,
			"user_id", user.UserID)

		if err := mgr.StopContainer(ctx, user.ContainerID); err != nil {
			slog.Error("TTL worker failed to stop container",
				"error", err,
				"container_id", user.ContainerID,
				"user_id", user.UserID)
		}

		if onCleanup != nil {
			onCleanup(user.UserID)
		}

		if err := updateContainerIDWithRetry(ctx, repo, user.UserID, "", user.ContainerID); err != nil {
			slog.Warn("TTL worker failed to clear container ID after retries",
				"error", err,
				"user_id", user.UserID)
		}

		// Delete agent session with retry logic to handle SQLITE_BUSY errors
		// This can occur when the debounced writer is still flushing
		if err := deleteAgentSessionWithRetry(ctx, repo, user.UserID); err != nil {
			slog.Warn("TTL worker failed to delete agent session after retries",
				"error", err,
				"user_id", user.UserID)
		}
	}

	slog.Info("TTL worker cleanup completed", "cleaned", len(expiredUsers))

	if deleted, err := repo.CleanupExpiredSessions(ctx, 7*24*time.Hour); err != nil {
		slog.Error("TTL worker failed to cleanup orphaned agent sessions", "error", err)
	} else if deleted > 0 {
		slog.Info("TTL worker cleaned up orphaned agent sessions", "count", deleted)
	}
}
