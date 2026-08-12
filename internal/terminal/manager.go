// Package terminal provides WebSocket-based terminal session management.
package terminal

import (
	"context"
	"log/slog"
	"sync"

	"github.com/coder/websocket"
)

// SafeConn provides thread-safe writes and JSON serialization to a WebSocket connection.
type SafeConn interface {
	WriteBinary(ctx context.Context, p []byte) error
	WriteText(ctx context.Context, p []byte) error
	WriteJSON(ctx context.Context, v interface{}) error
	Close(code websocket.StatusCode, reason string) error
}

// SessionManager manages active WebSocket connections for users.
type SessionManager struct {
	mu     sync.RWMutex
	active map[string]map[string]SafeConn
}

// NewSessionManager creates a new session manager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		active: make(map[string]map[string]SafeConn),
	}
}

// GetActive returns the active connection for a user and session.
func (m *SessionManager) GetActive(userID, sessionID string) SafeConn {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if sessions, ok := m.active[userID]; ok {
		return sessions[sessionID]
	}
	return nil
}

// Register adds a new WebSocket connection for a user/session.
func (m *SessionManager) Register(userID, sessionID string, conn SafeConn) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.active[userID]; !exists {
		m.active[userID] = make(map[string]SafeConn)
	}

	if existing, exists := m.active[userID][sessionID]; exists && existing != conn {
		if err := existing.Close(websocket.StatusNormalClosure, "session replaced"); err != nil {
			slog.Debug("Failed to close replaced terminal session", "user_id", userID, "session_id", sessionID, "error", err)
		}
	}

	m.active[userID][sessionID] = conn
	slog.Info("Terminal session registered", "user_id", userID, "session_id", sessionID)
}

// Unregister removes a WebSocket connection for a user/session.
func (m *SessionManager) Unregister(userID, sessionID string, conn SafeConn) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sessions, ok := m.active[userID]; ok {
		if current, exists := sessions[sessionID]; exists && current == conn {
			delete(sessions, sessionID)
			if len(sessions) == 0 {
				delete(m.active, userID)
			}
			slog.Info("Terminal session unregistered", "user_id", userID, "session_id", sessionID)
		}
	}
}

// BroadcastJSON sends a JSON message to all active terminal sessions for a user.
func (m *SessionManager) BroadcastJSON(userID string, v interface{}) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions, ok := m.active[userID]
	if !ok {
		return
	}

	for _, safe := range sessions {
		if err := safe.WriteJSON(context.Background(), v); err != nil {
			slog.Debug("Failed to broadcast JSON to terminal session", "user_id", userID, "error", err)
		}
	}
}

// CloseSession forcefully terminates all active sessions for a user.
func (m *SessionManager) CloseSession(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessions, ok := m.active[userID]
	if !ok {
		return
	}

	for sid, conn := range sessions {
		if err := conn.Close(websocket.StatusNormalClosure, "session closed"); err != nil {
			slog.Debug("Failed to close terminal session", "user_id", userID, "session_id", sid, "error", err)
		}
		slog.Info("Terminal session closed", "user_id", userID, "session_id", sid)
	}
	delete(m.active, userID)
}
