package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/ashureev/shsh-labs/internal/container"
	"github.com/ashureev/shsh-labs/internal/identity"
	"github.com/ashureev/shsh-labs/internal/store"
	"github.com/coder/websocket"
)

// WebSocketHandler handles WebSocket-based terminal sessions.
type WebSocketHandler struct {
	repo          store.Repository
	mgr           container.Manager
	sm            *SessionManager
	monitor       *TerminalMonitor
	allowedOrigin string
	isDev         bool
}

// NewWebSocketHandler creates a new WebSocket handler.
func NewWebSocketHandler(repo store.Repository, mgr container.Manager, sm *SessionManager, allowedOrigin string, isDev bool) *WebSocketHandler {
	return &WebSocketHandler{
		repo:          repo,
		mgr:           mgr,
		sm:            sm,
		allowedOrigin: allowedOrigin,
		isDev:         isDev,
	}
}

// SetMonitor sets the terminal monitor for proactive AI monitoring.
func (h *WebSocketHandler) SetMonitor(monitor *TerminalMonitor) {
	h.monitor = monitor
}

// wsWriter adapts websocket.Conn to io.Writer.
// Uses context.Background() for writes since WebSocket library handles its own
// connection state. The passed context is only for initial setup.
type wsWriter struct {
	conn *websocket.Conn
	ctx  context.Context
}

func (w *wsWriter) Write(p []byte) (int, error) {
	// Check if context is already cancelled before attempting write
	if w.ctx.Err() != nil {
		return 0, w.ctx.Err()
	}

	if err := w.conn.Write(context.Background(), websocket.MessageBinary, p); err != nil {
		// Check if this is a closed connection error - these are expected
		// when clients disconnect abruptly
		if w.ctx.Err() != nil {
			// Context cancelled, connection closing - this is expected
			return 0, w.ctx.Err()
		}
		// Only log unexpected errors
		slog.Debug("WebSocket write error", "error", err)
		return 0, err
	}
	return len(p), nil
}

// wsMessage represents WebSocket message structure.
type wsMessage struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	Cols    uint   `json:"cols,omitempty"`
	Rows    uint   `json:"rows,omitempty"`
}

// ServeHTTP implements http.Handler for WebSocket upgrade.
func (h *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID := identity.UserIDFromContext(r.Context())
	sessionID := identity.SessionIDFromContext(r.Context())
	slog.Info("WebSocket connection request", "user_id", userID, "session_id", sessionID, "ip", r.RemoteAddr)

	if !h.checkOrigin(r) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}

	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		slog.Error("Failed to accept WebSocket", "error", err, "user_id", userID)
		return
	}
	defer func() {
		if closeErr := ws.Close(websocket.StatusNormalClosure, "session ended"); closeErr != nil {
			slog.Debug("Failed to close websocket", "error", closeErr, "user_id", userID)
		}
	}()

	h.sm.Register(userID, sessionID, ws)
	defer h.sm.Unregister(userID, sessionID, ws)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	user, err := h.repo.GetUser(ctx, userID)
	if err != nil || user == nil || user.ContainerID == "" {
		slog.Warn("Container not ready", "user_id", userID)
		if err := h.writeJSON(ws, map[string]string{"error": "container_not_ready"}); err != nil {
			slog.Debug("Failed to send container_not_ready error", "error", err)
		}
		return
	}

	slog.Info("Attaching to container", "container_id", user.ContainerID, "user_id", userID)
	execID, execStream, err := h.mgr.CreateExecSession(ctx, user.ContainerID)
	if err != nil {
		slog.Error("Failed to create exec session", "error", err)
		if err := h.writeJSON(ws, map[string]string{"error": "failed_to_create_exec"}); err != nil {
			slog.Debug("Failed to send failed_to_create_exec error", "error", err)
		}
		return
	}
	defer func() {
		if closeErr := execStream.Close(); closeErr != nil {
			slog.Debug("Failed to close exec stream", "error", closeErr, "user_id", userID)
		}
	}()

	// Register session with terminal monitor for AI monitoring
	if h.monitor != nil {
		h.monitor.RegisterSession(userID, sessionID, user.ContainerID, user.VolumePath)
		defer h.monitor.UnregisterSession(userID, sessionID)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Input loop: WebSocket -> container.
	go func() {
		defer wg.Done()
		defer cancel()
		h.inputLoop(ctx, ws, execStream, userID, sessionID, execID)
	}()

	// Output loop: container -> WebSocket.
	go func() {
		defer wg.Done()
		defer cancel()
		h.outputLoop(ctx, ws, execStream, userID)
	}()

	wg.Wait()
	slog.Info("Terminal session ended", "user_id", userID)
}

func (h *WebSocketHandler) checkOrigin(r *http.Request) bool {
	if h.isDev {
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" || h.allowedOrigin == "*" {
		return true
	}
	if origin == h.allowedOrigin {
		return true
	}
	slog.Warn("WebSocket origin rejected", "origin", origin, "allowed", h.allowedOrigin)
	return false
}

//nolint:gocognit // Message dispatch must coordinate websocket, terminal, and monitor state.
func (h *WebSocketHandler) inputLoop(ctx context.Context, ws *websocket.Conn, execStream io.Writer, userID, sessionID, execID string) {
	slog.Debug("Starting input loop", "user_id", userID)
	for {
		_, message, err := ws.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) != -1 {
				slog.Debug("WebSocket closed by client", "user_id", userID)
			} else {
				slog.Warn("WebSocket read error", "error", err, "user_id", userID)
			}
			return
		}

		var msg wsMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			// Fallback to raw data.
			if _, err := execStream.Write(message); err != nil {
				slog.Error("Exec stream write error", "error", err)
				return
			}
			continue
		}

		switch msg.Type {
		case "data":
			// Send to container
			if _, err := execStream.Write([]byte(msg.Content)); err != nil {
				slog.Error("Exec stdin write error", "error", err)
				return
			}

			// Also process through terminal monitor for command detection
			// Skip if in editor mode - editor keystrokes are not shell commands
			if h.monitor != nil {
				inEditor := h.monitor.IsInEditorMode(userID, sessionID)
				slog.Debug("[WS] Editor mode check", "user_id", userID, "session_id", sessionID, "in_editor", inEditor, "content", msg.Content)
				if !inEditor {
					h.monitor.ProcessInput(ctx, userID, sessionID, []byte(msg.Content))
				}
			}
		case "ping":
			if err := h.writeJSON(ws, map[string]string{"type": "pong"}); err != nil {
				slog.Debug("Failed to send pong", "error", err)
			}
		case "resize":
			if err := h.mgr.ResizeExecSession(ctx, execID, msg.Cols, msg.Rows); err != nil {
				slog.Warn("Failed to resize", "error", err)
			}
		case "terminate":
			slog.Info("Terminal terminate requested", "user_id", userID, "session_id", sessionID)
			if err := h.writeJSON(ws, map[string]string{"type": "terminated"}); err != nil {
				slog.Debug("Failed to send terminated acknowledgment", "error", err)
			}
			return
		}

		// Update last seen asynchronously with timeout.
		go func() {
			updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := h.repo.UpdateLastSeen(updateCtx, userID, time.Now()); err != nil {
				slog.Warn("Failed to update last seen", "error", err)
			}
		}()
	}
}

func (h *WebSocketHandler) outputLoop(ctx context.Context, ws *websocket.Conn, execStream io.Reader, userID string) {
	sessionID := identity.SessionIDFromContext(ctx)
	sessionKey := userID + ":" + sessionID
	wsWriter := &wsWriter{ws, ctx}

	if h.monitor != nil {
		// Use async dual writer to prevent blocking WebSocket I/O
		writer := NewAsyncDualWriter(wsWriter, h.monitor, userID, sessionID, sessionKey, nil)
		defer func() {
			if closeErr := writer.Close(); closeErr != nil {
				slog.Debug("Failed to close async dual writer", "error", closeErr, "user_id", userID)
			}
		}()
		_, err := io.Copy(writer, execStream)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
			slog.Warn("Container output error", "error", err)
		}
	} else {
		_, err := io.Copy(wsWriter, execStream)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
			slog.Warn("Container output error", "error", err)
		}
	}
}

func (h *WebSocketHandler) writeJSON(ws *websocket.Conn, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return ws.Write(context.Background(), websocket.MessageText, data)
}
