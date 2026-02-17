// Package domain contains core domain types for the SHSH application.
package domain

import (
	"time"
)

// User represents a user in the system with their associated container state.
type User struct {
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	ContainerID string    `json:"container_id,omitempty"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	VolumePath  string    `json:"volume_path"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// HasActiveContainer returns true if the user has a non-empty container ID.
func (u *User) HasActiveContainer() bool {
	return u.ContainerID != ""
}

// SessionTTL returns the time until the container expires.
// Returns 0 if the container has already expired.
func (u *User) SessionTTL(sessionDuration time.Duration) time.Duration {
	if !u.HasActiveContainer() {
		return 0
	}
	expiresAt := u.LastSeenAt.Add(sessionDuration)
	ttl := time.Until(expiresAt)
	if ttl < 0 {
		return 0
	}
	return ttl
}
