package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ashureev/shsh-labs/internal/container"
	"github.com/ashureev/shsh-labs/internal/domain"
	"github.com/ashureev/shsh-labs/internal/identity"
	"github.com/ashureev/shsh-labs/internal/store"
	"github.com/coder/websocket"
)

// EventHandler defines the interface for components listening to shell events.
type EventHandler interface {
	HandleShellEvent(event ShellEvent, userID, sessionID, containerID string)
}

// WebSocketHandler handles WebSocket-based terminal sessions.
type WebSocketHandler struct {
	repo          store.Repository
	mgr           container.Manager
	sm            *SessionManager
	eventHandler  EventHandler
	allowedOrigin string
	isDev         bool
}

// NewWebSocketHandler creates a new WebSocket handler.
func NewWebSocketHandler(repo store.Repository, mgr container.Manager, sm *SessionManager, eventHandler EventHandler, allowedOrigin string, isDev bool) *WebSocketHandler {
	return &WebSocketHandler{
		repo:          repo,
		mgr:           mgr,
		sm:            sm,
		eventHandler:  eventHandler,
		allowedOrigin: allowedOrigin,
		isDev:         isDev,
	}
}

// safeWSConn provides thread-safe writes to a websocket.Conn.
type safeWSConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
	ctx  context.Context
}

func (s *safeWSConn) WriteBinary(ctx context.Context, p []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return s.ctx.Err()
	}
	return s.conn.Write(ctx, websocket.MessageBinary, p)
}

func (s *safeWSConn) WriteText(ctx context.Context, p []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return s.ctx.Err()
	}
	return s.conn.Write(ctx, websocket.MessageText, p)
}

func (s *safeWSConn) WriteJSON(ctx context.Context, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.WriteText(ctx, data)
}

func (s *safeWSConn) Close(code websocket.StatusCode, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.Close(code, reason)
}

type wsWriter struct {
	safe *safeWSConn
}

func (w *wsWriter) Write(p []byte) (int, error) {
	if err := w.safe.WriteBinary(context.Background(), p); err != nil {
		return 0, err
	}
	return len(p), nil
}

type wsMessage struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	Cols    uint   `json:"cols,omitempty"`
	Rows    uint   `json:"rows,omitempty"`
}

func (h *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID := identity.UserIDFromContext(r.Context())
	sessionID := identity.SessionIDFromContext(r.Context())
	// Parse terminal dimensions advertised by the browser at connect time.
	initCols := parseUintParam(r.URL.Query().Get("cols"), 80)
	initRows := parseUintParam(r.URL.Query().Get("rows"), 24)
	slog.Info("WebSocket connection request", "user_id", userID, "session_id", sessionID, "ip", r.RemoteAddr, "cols", initCols, "rows", initRows)

	if !h.checkOrigin(r) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}

	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{})
	if err != nil {
		slog.Error("Failed to accept WebSocket", "error", err, "user_id", userID)
		return
	}
	defer func() {
		_ = ws.Close(websocket.StatusNormalClosure, "session ended")
	}()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	safe := &safeWSConn{conn: ws, ctx: ctx}

	h.sm.Register(userID, sessionID, safe)
	defer h.sm.Unregister(userID, sessionID, safe)

	user, err := h.repo.GetUser(ctx, userID)
	if err != nil || user == nil {
		slog.Warn("User not found for terminal attach", "user_id", userID)
		_ = safe.WriteJSON(ctx, map[string]string{"error": "user_not_found"})
		return
	}

	containerID := user.ContainerID
	// Auto-heal: Ensure container is created/running if missing or stopped
	if containerID == "" {
		slog.Info("Auto-provisioning container for terminal connection", "user_id", userID)
		newID, ensureErr := h.mgr.EnsureContainer(ctx, userID, "", user.LastSeenAt, nil)
		if ensureErr != nil {
			slog.Error("Failed to auto-provision container", "error", ensureErr, "user_id", userID)
			_ = safe.WriteJSON(ctx, map[string]string{"error": "container_not_ready"})
			return
		}
		containerID = newID
		_ = h.repo.UpdateContainerID(ctx, userID, containerID, "")
	}

	slog.Info("Attaching to container", "container_id", containerID, "user_id", userID)
	execID, execStream, err := h.mgr.CreateExecSession(ctx, containerID, initCols, initRows)
	if err != nil {
		// If exec creation failed because container was dead/stopped, attempt one recovery
		slog.Warn("Exec session creation failed, attempting container recovery", "error", err, "container_id", containerID)
		recoveredID, ensureErr := h.mgr.EnsureContainer(ctx, userID, containerID, user.LastSeenAt, nil)
		if ensureErr != nil {
			slog.Error("Failed to recover container", "error", ensureErr)
			_ = safe.WriteJSON(ctx, map[string]string{"error": "failed_to_create_exec"})
			return
		}
		containerID = recoveredID
		_ = h.repo.UpdateContainerID(ctx, userID, containerID, "")
		execID, execStream, err = h.mgr.CreateExecSession(ctx, containerID, initCols, initRows)
		if err != nil {
			slog.Error("Failed to create exec session after recovery", "error", err)
			_ = safe.WriteJSON(ctx, map[string]string{"error": "failed_to_create_exec"})
			return
		}
	}
	defer func() {
		_ = execStream.Close()
	}()

	// Initialize Telemetry Parser for this session
	telemetryParser := NewTelemetryParser()
	telemetryParser.OnEvent = func(event ShellEvent) {
		if event.Type == EventCommandEnd {
			cmd := strings.TrimSpace(event.Command)
			if cmd == "" || strings.HasPrefix(cmd, "_shsh") || strings.HasPrefix(cmd, "[ ") || strings.HasPrefix(cmd, "test ") {
				return
			}

			// Record command in SQLite
			_ = h.repo.SaveCommand(context.Background(), &domain.CommandLog{
				UserID:     userID,
				SessionID:  sessionID,
				Command:    cmd,
				PWD:        event.PWD,
				ExitCode:   event.ExitCode,
				DurationMs: event.Duration.Milliseconds(),
				CreatedAt:  time.Now(),
			})
		}

		// Forward event to AI tutor debounce loop
		if h.eventHandler != nil {
			h.eventHandler.HandleShellEvent(event, userID, sessionID, user.ContainerID)
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Input loop: WebSocket -> container
	go func() {
		defer wg.Done()
		defer cancel()
		h.inputLoop(ctx, ws, safe, execStream, userID, sessionID, execID)
	}()

	// Output loop: container -> WebSocket & TelemetryParser
	go func() {
		defer wg.Done()
		defer cancel()
		h.outputLoop(ctx, safe, execStream, telemetryParser)
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
	return origin == h.allowedOrigin
}

func (h *WebSocketHandler) inputLoop(ctx context.Context, ws *websocket.Conn, safe *safeWSConn, execStream io.Writer, userID, sessionID, execID string) {
	lastSeenUpdate := time.Now()

	for {
		_, message, err := ws.Read(ctx)
		if err != nil {
			return
		}

		var msg wsMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			if _, err := execStream.Write(message); err != nil {
				return
			}
			continue
		}

		switch msg.Type {
		case "data":
			if _, err := execStream.Write([]byte(msg.Content)); err != nil {
				return
			}
		case "ping":
			_ = safe.WriteJSON(ctx, map[string]string{"type": "pong"})
		case "resize":
			if err := h.mgr.ResizeExecSession(ctx, execID, msg.Cols, msg.Rows); err != nil {
				slog.Warn("Failed to resize", "error", err)
			}
		case "terminate":
			_ = safe.WriteJSON(ctx, map[string]string{"type": "terminated"})
			return
		}

		// Throttle last_seen DB writes to once every 10s
		if time.Since(lastSeenUpdate) > 10*time.Second {
			lastSeenUpdate = time.Now()
			go func() {
				updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = h.repo.UpdateLastSeen(updateCtx, userID, time.Now())
			}()
		}
	}
}

func (h *WebSocketHandler) outputLoop(ctx context.Context, safe *safeWSConn, execStream io.Reader, parser *TelemetryParser) {
	writer := &wsWriter{safe: safe}
	buf := make([]byte, 4096)

	for {
		if ctx.Err() != nil {
			return
		}

		n, err := execStream.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			// 1. Forward raw bytes to browser terminal with thread-safe writer
			if _, wErr := writer.Write(chunk); wErr != nil {
				return
			}
			// 2. Feed chunk to deterministic telemetry parser
			if parser != nil {
				parser.Feed(chunk)
			}
		}

		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
				slog.Warn("Container output stream error", "error", err)
			}
			return
		}
	}
}

// parseUintParam parses a URL query parameter as an unsigned integer.
// Returns fallback if the string is empty, non-numeric, or zero.
func parseUintParam(s string, fallback uint) uint {
	if v, err := strconv.ParseUint(s, 10, 32); err == nil && v > 0 {
		return uint(v)
	}
	return fallback
}
