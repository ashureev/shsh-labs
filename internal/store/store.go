// Package store provides data persistence interfaces and implementations.
package store

import (
	"context"
	"time"

	"github.com/ashureev/shsh-labs/internal/domain"
)

// Repository defines the interface for persisting user and container data.
type Repository interface {
	// GetUser retrieves a user by their user ID.
	GetUser(ctx context.Context, userID string) (*domain.User, error)

	// UpsertUser creates or updates a user record.
	UpsertUser(ctx context.Context, user *domain.User) error

	// UpdateLastSeen updates the last_seen_at timestamp for a user.
	UpdateLastSeen(ctx context.Context, userID string, lastSeen time.Time) error

	// UpdateContainerID updates the container_id for a user.
	// If expectedID is non-empty, the update will only happen if the current
	// container_id matches expectedID (optimistic locking).
	UpdateContainerID(ctx context.Context, userID string, containerID string, expectedID string) error

	// GetExpiredSessions retrieves users whose containers have exceeded the inactivity TTL.
	GetExpiredSessions(ctx context.Context, ttl time.Duration) ([]*domain.User, error)

	// Ping verifies database connectivity and returns an error if the database is unreachable.
	Ping(ctx context.Context) error

	// Close closes the database connection.
	Close() error

	// GetAgentSession retrieves agent session state for a user.
	GetAgentSession(ctx context.Context, userID string) (*domain.AgentSession, error)

	// UpsertAgentSession creates or updates agent session state.
	UpsertAgentSession(ctx context.Context, session *domain.AgentSession) error

	// DeleteAgentSession removes agent session state.
	DeleteAgentSession(ctx context.Context, userID string) error

	// CleanupExpiredSessions removes sessions older than TTL.
	CleanupExpiredSessions(ctx context.Context, ttl time.Duration) (int64, error)

	// DeleteLegacyLocalState removes legacy single-user records.
	DeleteLegacyLocalState(ctx context.Context) (usersDeleted int64, agentSessionsDeleted int64, err error)
}
