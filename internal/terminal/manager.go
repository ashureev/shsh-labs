// Package terminal provides WebSocket-based terminal session management.
package terminal

import (
	"log/slog"
	"sync"

	"github.com/coder/websocket"
)

// SessionManager manages active WebSocket connections for users.
type SessionManager struct {
	mu     sync.RWMutex
	active map[string]map[string]*websocket.Conn
}

// NewSessionManager creates a new session manager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		active: make(map[string]map[string]*websocket.Conn),
	}
}

// GetActive returns the active connection for a user and session.
func (m *SessionManager) GetActive(userID, sessionID string) *websocket.Conn {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if sessions, ok := m.active[userID]; ok {
		return sessions[sessionID]
	}
	return nil
}

// Register adds a new WebSocket connection for a user/session.
func (m *SessionManager) Register(userID, sessionID string, conn *websocket.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.active[userID]; !exists {
		m.active[userID] = make(map[string]*websocket.Conn)
	}

	if existing, exists := m.active[userID][sessionID]; exists && existing != conn {
		_ = existing.Close(websocket.StatusNormalClosure, "session replaced")
	}

	m.active[userID][sessionID] = conn
	slog.Info("Terminal session registered", "user_id", userID, "session_id", sessionID)
}

// Unregister removes a WebSocket connection for a user/session.
func (m *SessionManager) Unregister(userID, sessionID string, conn *websocket.Conn) {
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

// CloseSession forcefully terminates all active sessions for a user.
func (m *SessionManager) CloseSession(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessions, ok := m.active[userID]
	if !ok {
		return
	}

	for sid, conn := range sessions {
		_ = conn.Close(websocket.StatusNormalClosure, "session closed")
		slog.Info("Terminal session closed", "user_id", userID, "session_id", sid)
	}
	delete(m.active, userID)
}
