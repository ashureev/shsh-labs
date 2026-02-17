package agent

import (
	"container/list"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ashureev/shsh-labs/internal/identity"
	"github.com/ashureev/shsh-labs/internal/store"
	"github.com/docker/docker/client"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

// maxRequestBodySize is the maximum allowed request body size (1MB).
const maxRequestBodySize = 1 << 20 // 1MB

// SSEConnection represents a single SSE client connection.
type SSEConnection struct {
	ID          int64
	UserID      string
	SessionID   string
	EventID     int64
	ConnectedAt time.Time
	LastEventID int64
	Writer      http.ResponseWriter
	Flusher     http.Flusher
	Done        chan struct{}
	mu          sync.Mutex
}

// SSEMessageQueue buffers messages for disconnected clients.
type SSEMessageQueue struct {
	mu       sync.RWMutex
	messages *list.List // List of *QueuedMessage
	maxSize  int
}

// QueuedMessage represents a message in the queue.
type QueuedMessage struct {
	EventID   int64
	UserID    string
	SessionID string
	Response  *Response
	Timestamp time.Time
}

// NewSSEMessageQueue creates a new message queue with specified max size.
func NewSSEMessageQueue(maxSize int) *SSEMessageQueue {
	if maxSize <= 0 {
		maxSize = 100 // Default: keep last 100 messages per user
	}
	return &SSEMessageQueue{
		messages: list.New(),
		maxSize:  maxSize,
	}
}

// Enqueue adds a message to the queue.
func (q *SSEMessageQueue) Enqueue(userID, sessionID string, eventID int64, resp *Response) {
	q.mu.Lock()
	defer q.mu.Unlock()

	msg := &QueuedMessage{
		EventID:   eventID,
		UserID:    userID,
		SessionID: sessionID,
		Response:  resp,
		Timestamp: time.Now(),
	}

	q.messages.PushBack(msg)

	// Remove old messages if queue is too large
	for q.messages.Len() > q.maxSize {
		q.messages.Remove(q.messages.Front())
	}
}

// GetMissedMessages retrieves messages after a specific event ID for a user.
func (q *SSEMessageQueue) GetMissedMessages(userID, sessionID string, afterEventID int64) []*QueuedMessage {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var missed []*QueuedMessage
	for e := q.messages.Front(); e != nil; e = e.Next() {
		msg := e.Value.(*QueuedMessage)
		if msg.UserID == userID && msg.SessionID == sessionID && msg.EventID > afterEventID {
			missed = append(missed, msg)
		}
	}
	return missed
}

// Handler handles AI agent HTTP requests with robust SSE support.
type Handler struct {
	agent          *Service
	dockerClient   *client.Client
	repo           store.Repository
	rateLimiter    *RateLimiter
	broadcastChan  chan *Response
	sseConnections map[string]map[int64]*SSEConnection // sessionKey -> ConnectionID -> Connection
	messageQueue   *SSEMessageQueue
	connectionsMu  sync.RWMutex
	eventCounter   int64
	connectionID   int64 // Counter for unique connection IDs
	counterMu      sync.Mutex
	done           chan struct{} // Closed to signal goroutine shutdown
	log            ConversationLogger
}

func sseSessionKey(userID, sessionID string) string {
	return userID + ":" + sessionID
}

// RateLimiter implements a simple per-session rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow checks if a request is allowed for the given key.
func (r *RateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	var recent []time.Time
	for _, t := range r.requests[key] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}

	if len(recent) >= r.limit {
		r.requests[key] = recent
		return false
	}

	r.requests[key] = append(recent, now)
	return true
}

// NewHandlerWithGrpcClient creates a new agent handler using the gRPC client.
func NewHandlerWithGrpcClient(dockerClient *client.Client, repo store.Repository, broadcastChan chan *Response, grpcClient *GrpcClient, conversationLogger ConversationLogger) (*Handler, error) {
	agentService, err := NewServiceWithProcessor(grpcClient)
	if err != nil {
		return nil, err
	}

	return newHandlerWithService(dockerClient, repo, broadcastChan, agentService, conversationLogger), nil
}

// newHandlerWithService creates a handler with the given agent service.
func newHandlerWithService(dockerClient *client.Client, repo store.Repository, broadcastChan chan *Response, agentService *Service, conversationLogger ConversationLogger) *Handler {
	if conversationLogger == nil {
		conversationLogger = noopConversationLogger{}
	}
	handler := &Handler{
		agent:          agentService,
		dockerClient:   dockerClient,
		repo:           repo,
		rateLimiter:    NewRateLimiter(10, time.Minute),
		broadcastChan:  broadcastChan,
		sseConnections: make(map[string]map[int64]*SSEConnection),
		messageQueue:   NewSSEMessageQueue(100),
		done:           make(chan struct{}),
		log:            conversationLogger,
	}

	// Start the broadcaster goroutine
	go handler.broadcastLoop(broadcastChan)

	return handler
}

// HandleChat handles POST /api/agent/chat requests.
func (h *Handler) HandleChat(w http.ResponseWriter, r *http.Request) {
	userID := identity.UserIDFromContext(r.Context())
	sessionID := identity.SessionIDFromContext(r.Context())
	if userID == "" {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	user, err := h.repo.GetUser(r.Context(), userID)
	if err != nil || user == nil {
		http.Error(w, `{"error": "user not found"}`, http.StatusUnauthorized)
		return
	}

	if user.ContainerID == "" {
		http.Error(w, `{"error": "no active container"}`, http.StatusBadRequest)
		return
	}

	if user.VolumePath == "" {
		http.Error(w, `{"error": "no volume path"}`, http.StatusBadRequest)
		return
	}

	if !h.rateLimiter.Allow(sseSessionKey(user.UserID, sessionID)) {
		http.Error(w, `{"error": "rate limit exceeded"}`, http.StatusTooManyRequests)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if errors.Is(err, http.ErrBodyReadAfterClose) || errors.Is(err, http.ErrHandlerTimeout) {
			http.Error(w, `{"error": "request body too large"}`, http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, `{"error": "message is required"}`, http.StatusBadRequest)
		return
	}

	req.ContainerID = user.ContainerID
	req.VolumePath = user.VolumePath
	req.UserID = user.UserID
	req.SessionID = sessionID
	reqID := chiMiddleware.GetReqID(r.Context())

	slog.Info("Agent chat request",
		"user_id", user.UserID,
		"session_id", sessionID,
		"container_id", req.ContainerID,
		"message_length", len(req.Message),
	)
	h.log.Log(ConversationLogEvent{
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
		UserID:     req.UserID,
		SessionID:  req.SessionID,
		Channel:    "chat_http",
		Direction:  "outbound",
		EventType:  "chat_user_message",
		ContentRaw: req.Message,
		Content:    cleanForReadability(req.Message),
		Meta: map[string]any{
			"request_id": reqID,
		},
	})

	// Stream response via SSE.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error": "streaming not supported"}`, http.StatusInternalServerError)
		return
	}

	var assistantContent strings.Builder
	streamChunks := 0
	partial := false
	streamErrMsg := ""

	for resp, err := range h.agent.Chat(r.Context(), req) {
		if err != nil {
			partial = true
			streamErrMsg = err.Error()
			slog.Error("Agent stream failed", "error", err)
			h.logAssistantMessage(req.UserID, req.SessionID, assistantContent.String(), streamChunks, partial, streamErrMsg, reqID)
			if writeErr := writeSSE(w, "error", err.Error()); writeErr != nil {
				slog.Warn("failed to write SSE error event", "error", writeErr)
				return
			}
			flusher.Flush()
			return
		}

		if resp != nil && resp.Response != "" {
			streamChunks++
			assistantContent.WriteString(resp.Response)
		}

		data, err := json.Marshal(resp)
		if err != nil {
			slog.Warn("failed to marshal chat response", "error", err)
			if writeErr := writeSSE(w, "error", "failed to serialize response"); writeErr != nil {
				slog.Warn("failed to write SSE serialization error", "error", writeErr)
			}
			flusher.Flush()
			return
		}
		if err := writeSSE(w, "message", string(data)); err != nil {
			slog.Warn("failed to write SSE message event", "error", err)
			partial = true
			streamErrMsg = err.Error()
			h.logAssistantMessage(req.UserID, req.SessionID, assistantContent.String(), streamChunks, partial, streamErrMsg, reqID)
			return
		}
		flusher.Flush()
	}
	h.logAssistantMessage(req.UserID, req.SessionID, assistantContent.String(), streamChunks, partial, streamErrMsg, reqID)
}

func (h *Handler) logAssistantMessage(userID, sessionID, content string, streamChunks int, partial bool, streamErrMsg, requestID string) {
	h.log.Log(ConversationLogEvent{
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
		UserID:     userID,
		SessionID:  sessionID,
		Channel:    "chat_http",
		Direction:  "inbound",
		EventType:  "chat_assistant_message",
		ContentRaw: content,
		Content:    cleanForReadability(content),
		Meta: map[string]any{
			"stream_chunks": streamChunks,
			"partial":       partial,
			"stream_error":  streamErrMsg,
			"request_id":    requestID,
		},
	})
}

// RegisterRoutes registers agent routes (requires authentication).
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/agent", func(r chi.Router) {
		r.Post("/chat", h.HandleChat)
		r.Get("/stream", h.HandleStream)
	})
}

// Close releases handler resources.
func (h *Handler) Close() {
	close(h.done)
	if h.agent != nil {
		h.agent.Close()
	}
	if h.log != nil {
		if err := h.log.Close(); err != nil {
			slog.Warn("failed to close conversation logger", "error", err)
		}
	}
}

// GetService returns the underlying agent service.
func (h *Handler) GetService() *Service {
	return h.agent
}

// broadcastLoop listens for messages and distributes them to connected clients.
func (h *Handler) broadcastLoop(broadcastChan chan *Response) {
	slog.Info("[BROADCAST] Broadcast loop started")
	for {
		select {
		case <-h.done:
			slog.Info("[BROADCAST] Broadcast loop shutting down")
			return
		case resp, ok := <-broadcastChan:
			if !ok {
				slog.Info("[BROADCAST] Broadcast channel closed, shutting down")
				return
			}
			if resp == nil {
				slog.Warn("[BROADCAST] Nil response received, skipping")
				continue
			}

			slog.Info("[BROADCAST] Received message",
				"user_id", resp.UserID,
				"type", resp.Type,
				"silent", resp.Silent,
				"content_len", len(resp.Content),
			)
			raw := resp.Sidebar
			if raw == "" {
				raw = resp.Content
			}
			h.log.Log(ConversationLogEvent{
				Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
				UserID:     resp.UserID,
				SessionID:  resp.SessionID,
				Channel:    "proactive_broadcast",
				Direction:  "inbound",
				EventType:  "proactive_message",
				ContentRaw: raw,
				Content:    cleanForReadability(raw),
				Meta: map[string]any{
					"response_type":   resp.Type,
					"silent":          resp.Silent,
					"require_confirm": resp.RequireConfirm,
					"block":           resp.Block,
					"alert":           resp.Alert,
					"pattern":         resp.Pattern,
					"tools_used":      resp.ToolsUsed,
				},
			})

			h.counterMu.Lock()
			h.eventCounter++
			eventID := h.eventCounter
			h.counterMu.Unlock()

			// Queue message for potential replay
			h.messageQueue.Enqueue(resp.UserID, resp.SessionID, eventID, resp)

			sessionKey := sseSessionKey(resp.UserID, resp.SessionID)
			// Send to all connected clients for this user/session (fan-out)
			h.connectionsMu.RLock()
			userConns, exists := h.sseConnections[sessionKey]
			if !exists {
				h.connectionsMu.RUnlock()
				slog.Warn("[BROADCAST] No connections found for session", "user_id", resp.UserID, "session_id", resp.SessionID)
				continue
			}

			// Snapshot connections to avoid holding RLock during writes
			conns := make([]*SSEConnection, 0, len(userConns))
			for _, c := range userConns {
				conns = append(conns, c)
			}
			h.connectionsMu.RUnlock()

			for _, conn := range conns {
				h.sendToConnection(conn, eventID, resp)
			}
		}
	}
}

// sendToConnection sends a message to a specific connection.
func (h *Handler) sendToConnection(conn *SSEConnection, eventID int64, resp *Response) {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	select {
	case <-conn.Done:
		return // Connection closed
	default:
	}

	data, err := json.Marshal(map[string]interface{}{
		"type":    resp.Type,
		"content": resp.Content,
		"sidebar": resp.Sidebar,
		"alert":   resp.Alert,
		"pattern": resp.Pattern,
	})
	if err != nil {
		slog.Error("[SEND] Failed to marshal SSE message", "error", err, "conn_id", conn.ID)
		return
	}

	// Write with event ID for replay capability
	_, err = fmt.Fprintf(conn.Writer, "id: %d\nevent: message\ndata: %s\n\n", eventID, string(data))
	if err != nil {
		slog.Error("[SEND] Failed to write to SSE connection",
			"error", err,
			"conn_id", conn.ID,
			"user_id", conn.UserID,
		)
		return
	}

	conn.Flusher.Flush()
	conn.EventID = eventID
}

// HandleStream handles SSE stream for proactive agent messages from terminal monitoring.
// This enhanced version includes:
// - Event ID tracking for message replay
// - Configured retry timing
// - Connection state management
// - Missed message recovery
func (h *Handler) HandleStream(w http.ResponseWriter, r *http.Request) {
	userID := identity.UserIDFromContext(r.Context())
	sessionID := identity.SessionIDFromContext(r.Context())
	streamKey := sseSessionKey(userID, sessionID)
	if userID == "" {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	user, err := h.repo.GetUser(r.Context(), userID)
	if err != nil || user == nil {
		http.Error(w, `{"error": "user not found"}`, http.StatusUnauthorized)
		return
	}

	if user.ContainerID == "" {
		http.Error(w, `{"error": "no active container"}`, http.StatusBadRequest)
		return
	}

	slog.Info("Agent stream connected", "user_id", user.UserID, "session_id", sessionID)

	// Parse Last-Event-ID header or query param for replay
	lastEventID := int64(0)
	idHeader := r.Header.Get("Last-Event-ID")
	if idHeader == "" {
		idHeader = r.URL.Query().Get("lastEventId")
	}
	if idHeader != "" {
		if parsed, err := strconv.ParseInt(idHeader, 10, 64); err == nil {
			lastEventID = parsed
			slog.Info("SSE client reconnecting with Last-Event-ID",
				"user_id", user.UserID,
				"last_event_id", lastEventID,
			)
		}
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error": "streaming not supported"}`, http.StatusInternalServerError)
		return
	}

	// Configure client retry behavior (5 seconds)
	if _, err := io.WriteString(w, "retry: 5000\n\n"); err != nil {
		slog.Warn("failed to write SSE retry header", "error", err, "user_id", user.UserID)
		return
	}
	flusher.Flush()

	// Create connection
	h.counterMu.Lock()
	h.connectionID++
	connID := h.connectionID
	h.counterMu.Unlock()

	conn := &SSEConnection{
		ID:          connID,
		UserID:      user.UserID,
		SessionID:   sessionID,
		ConnectedAt: time.Now(),
		LastEventID: lastEventID,
		Writer:      w,
		Flusher:     flusher,
		Done:        make(chan struct{}),
	}

	// Register connection
	h.connectionsMu.Lock()
	if _, exists := h.sseConnections[streamKey]; !exists {
		h.sseConnections[streamKey] = make(map[int64]*SSEConnection)
	}
	h.sseConnections[streamKey][connID] = conn
	h.connectionsMu.Unlock()

	defer func() {
		h.connectionsMu.Lock()
		if userConns, exists := h.sseConnections[streamKey]; exists {
			delete(userConns, connID)
			if len(userConns) == 0 {
				delete(h.sseConnections, streamKey)
			}
		}
		h.connectionsMu.Unlock()
		slog.Info("SSE connection closed", "user_id", user.UserID, "session_id", sessionID, "conn_id", connID)
	}()

	// Send missed messages if reconnecting
	if lastEventID > 0 {
		missed := h.messageQueue.GetMissedMessages(user.UserID, sessionID, lastEventID)
		if len(missed) > 0 {
			slog.Info("Sending missed messages",
				"user_id", user.UserID,
				"session_id", sessionID,
				"count", len(missed),
			)
			for _, msg := range missed {
				h.sendToConnection(conn, msg.EventID, msg.Response)
			}
		}
	}

	// Send initial connection event
	h.counterMu.Lock()
	h.eventCounter++
	eventID := h.eventCounter
	h.counterMu.Unlock()

	conn.EventID = eventID
	connectedData := fmt.Sprintf(`{"status":"connected","user_id":"%s","event_id":%d}`,
		user.UserID, eventID)
	if err := writeSSEWithID(w, eventID, "connected", connectedData); err != nil {
		slog.Warn("failed to write SSE connected event", "error", err, "user_id", user.UserID)
		return
	}
	flusher.Flush()

	slog.Info("SSE connection established",
		"user_id", user.UserID,
		"session_id", sessionID,
		"event_id", eventID,
		"reconnect", lastEventID > 0,
	)

	// Keepalive ticker (10 seconds, within WriteTimeout)
	keepalive := time.NewTicker(10 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			slog.Info("Agent stream disconnected", "user_id", user.UserID, "session_id", sessionID)
			return
		case <-conn.Done:
			slog.Info("SSE connection done signal", "user_id", user.UserID, "session_id", sessionID)
			return
		case <-keepalive.C:
			conn.mu.Lock()
			if err := writeSSE(w, "ping", `{"status":"alive"}`); err != nil {
				conn.mu.Unlock()
				slog.Warn("failed to write SSE keepalive ping", "error", err, "user_id", user.UserID)
				return
			}
			flusher.Flush()
			conn.mu.Unlock()
		}
	}
}

func writeSSE(w io.Writer, event, data string) error {
	_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	return err
}

func writeSSEWithID(w io.Writer, id int64, event, data string) error {
	_, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", id, event, data)
	return err
}
