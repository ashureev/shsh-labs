package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/ashureev/shsh-labs/internal/domain"
	"github.com/ashureev/shsh-labs/internal/identity"
	"github.com/ashureev/shsh-labs/internal/llm"
	"github.com/ashureev/shsh-labs/internal/store"
	"github.com/ashureev/shsh-labs/internal/terminal"
	"github.com/ashureev/shsh-labs/internal/tutor"
	"github.com/go-chi/chi/v5"
)

// TutorHandler manages AI tutor interactions and SSE streaming.
type TutorHandler struct {
	tutorEngine *tutor.Engine
	repo        store.Repository
	sm          *terminal.SessionManager
	clientsMu   sync.RWMutex
	clients     map[string]map[chan tutor.HintEvent]struct{} // userID -> set of client channels
}

// NewTutorHandler creates a new TutorHandler instance.
func NewTutorHandler(engine *tutor.Engine, repo store.Repository, sm *terminal.SessionManager) *TutorHandler {
	h := &TutorHandler{
		tutorEngine: engine,
		repo:        repo,
		sm:          sm,
		clients:     make(map[string]map[chan tutor.HintEvent]struct{}),
	}

	// Register callback to broadcast hints to connected WebSocket and SSE clients
	engine.OnHint = func(hint tutor.HintEvent) {
		// 1. Send directly down the user's active WebSocket connection
		if sm != nil {
			sm.BroadcastJSON(hint.UserID, map[string]interface{}{
				"type":       "hint",
				"content":    hint.Content,
				"command":    hint.Command,
				"exit_code":  hint.ExitCode,
				"tools_used": hint.ToolsUsed,
				"proactive":  true,
			})
		}

		// 2. Broadcast to any SSE subscribers
		h.broadcastHint(hint)

		// 3. Persist hint as an assistant message
		if err := repo.SaveChatMessage(context.Background(), &domain.ChatMessage{
			UserID:    hint.UserID,
			SessionID: hint.SessionID,
			Role:      "assistant",
			Content:   hint.Content,
			CreatedAt: hint.Timestamp,
		}); err != nil {
			slog.Debug("Failed to save hint message to store", "error", err)
		}
	}

	return h
}

// RegisterRoutes sets up tutor-related endpoints on the API subrouter.
func (h *TutorHandler) RegisterRoutes(r chi.Router) {
	r.Get("/tutor/stream", h.ServeSSE)
	r.Post("/chat", h.HandleChat)
	r.Get("/chat", h.GetChatHistory)
	r.Get("/commands", h.GetCommands)
}

// broadcastHint sends a hint to all active SSE subscribers for that user.
func (h *TutorHandler) broadcastHint(hint tutor.HintEvent) {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	userClients, exists := h.clients[hint.UserID]
	if !exists {
		return
	}

	for ch := range userClients {
		select {
		case ch <- hint:
		default:
			// Non-blocking drop if client buffer is full
		}
	}
}

// ServeSSE handles the Server-Sent Events stream for ambient tutor nudges.
func (h *TutorHandler) ServeSSE(w http.ResponseWriter, r *http.Request) {
	userID := identity.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "local"
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)

	clientCh := make(chan tutor.HintEvent, 10)

	h.clientsMu.Lock()
	if _, exists := h.clients[userID]; !exists {
		h.clients[userID] = make(map[chan tutor.HintEvent]struct{})
	}
	h.clients[userID][clientCh] = struct{}{}
	h.clientsMu.Unlock()

	defer func() {
		h.clientsMu.Lock()
		if userClients, exists := h.clients[userID]; exists {
			delete(userClients, clientCh)
			if len(userClients) == 0 {
				delete(h.clients, userID)
			}
		}
		h.clientsMu.Unlock()
	}()

	// Send initial connection heartbeat
	fmt.Fprintf(w, "event: connected\ndata: {\"status\": \"ready\"}\n\n")
	flusher.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		case hint := <-clientCh:
			data, err := json.Marshal(hint)
			if err == nil {
				fmt.Fprintf(w, "event: hint\ndata: %s\n\n", string(data))
				flusher.Flush()
			}
		}
	}
}

type chatRequest struct {
	Message string `json:"message"`
}

// HandleChat processes questions from the user to the AI mentor.
func (h *TutorHandler) HandleChat(w http.ResponseWriter, r *http.Request) {
	userID := identity.UserIDFromContext(r.Context())
	sessionID := identity.SessionIDFromContext(r.Context())

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Message == "" {
		Error(w, http.StatusBadRequest, "invalid message")
		return
	}

	user, err := h.repo.GetUser(r.Context(), userID)
	containerID := ""
	if err == nil && user != nil {
		containerID = user.ContainerID
	}

	// Persist user question
	_ = h.repo.SaveChatMessage(r.Context(), &domain.ChatMessage{
		UserID:    userID,
		SessionID: sessionID,
		Role:      "user",
		Content:   req.Message,
		CreatedAt: time.Now(),
	})

	// Fetch recent history
	dbHistory, _ := h.repo.GetChatHistory(r.Context(), userID, sessionID, 10)
	var llmHistory []llm.Message
	for _, m := range dbHistory {
		role := llm.RoleUser
		if m.Role == "assistant" {
			role = llm.RoleAssistant
		}
		llmHistory = append(llmHistory, llm.Message{
			Role:    role,
			Content: m.Content,
		})
	}

	// Fetch recent terminal commands for live context
	recentCommands, _ := h.repo.GetRecentCommands(r.Context(), userID, sessionID, 15)

	answer, toolsUsed, err := h.tutorEngine.Ask(r.Context(), userID, sessionID, containerID, req.Message, llmHistory, recentCommands)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Persist assistant answer
	toolsJSON, _ := json.Marshal(toolsUsed)
	_ = h.repo.SaveChatMessage(r.Context(), &domain.ChatMessage{
		UserID:    userID,
		SessionID: sessionID,
		Role:      "assistant",
		Content:   answer,
		ToolsJSON: string(toolsJSON),
		CreatedAt: time.Now(),
	})

	JSON(w, http.StatusOK, map[string]interface{}{
		"answer":     answer,
		"tools_used": toolsUsed,
	})
}

// GetChatHistory returns recent chat messages.
func (h *TutorHandler) GetChatHistory(w http.ResponseWriter, r *http.Request) {
	userID := identity.UserIDFromContext(r.Context())
	sessionID := identity.SessionIDFromContext(r.Context())

	messages, err := h.repo.GetChatHistory(r.Context(), userID, sessionID, 50)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to get chat history")
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"messages": messages,
	})
}

// GetCommands returns recent command timeline entries.
func (h *TutorHandler) GetCommands(w http.ResponseWriter, r *http.Request) {
	userID := identity.UserIDFromContext(r.Context())
	sessionID := identity.SessionIDFromContext(r.Context())

	commands, err := h.repo.GetRecentCommands(r.Context(), userID, sessionID, 50)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to get commands")
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"commands": commands,
	})
}
