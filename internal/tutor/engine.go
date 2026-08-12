package tutor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ashureev/shsh-labs/internal/domain"
	"github.com/ashureev/shsh-labs/internal/llm"
	"github.com/ashureev/shsh-labs/internal/terminal"
)

// HintEvent represents a proactive hint or response to be streamed to the client UI.
type HintEvent struct {
	Type        string    `json:"type"` // "nudge", "concept", "chat", "tool_inspection"
	Content     string    `json:"content"`
	Command     string    `json:"command,omitempty"`
	ExitCode    int       `json:"exit_code,omitempty"`
	ToolsUsed   []string  `json:"tools_used,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	UserID      string    `json:"user_id"`
	SessionID   string    `json:"session_id"`
}

// Config controls debounce timers and behavior.
type Config struct {
	DebounceDuration time.Duration
	EnableProactive  bool
}

// DefaultConfig returns recommended production defaults.
func DefaultConfig() Config {
	return Config{
		DebounceDuration: 3000 * time.Millisecond,
		EnableProactive:  true,
	}
}

// pendingSession tracks active timers for ambient debouncing.
type pendingSession struct {
	timer       *time.Timer
	lastFailed  terminal.ShellEvent
	containerID string
}

// Engine coordinates the AI tutor, debounce loop, tool calling, and LLM communication.
type Engine struct {
	mu           sync.Mutex
	llm          llm.Provider
	runner       ContainerRunner
	registry     *ToolRegistry
	cfg          Config
	sessions     map[string]*pendingSession
	OnHint       func(HintEvent)
	OnToolAction func(name string, args string, result string)
}

// NewEngine creates an AI Tutor Engine.
func NewEngine(llmProvider llm.Provider, runner ContainerRunner, cfg Config) *Engine {
	if cfg.DebounceDuration == 0 {
		cfg.DebounceDuration = 3 * time.Second
	}

	return &Engine{
		llm:      llmProvider,
		runner:   runner,
		registry: NewToolRegistry(runner),
		cfg:      cfg,
		sessions: make(map[string]*pendingSession),
	}
}

// SetProvider dynamically switches the active LLM provider (e.g. from UI settings).
func (e *Engine) SetProvider(provider llm.Provider) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.llm = provider
}

// HandleShellEvent processes incoming shell telemetry events and manages the debounce window.
func (e *Engine) HandleShellEvent(event terminal.ShellEvent, userID, sessionID, containerID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := userID

	switch event.Type {
	case terminal.EventCommandStart:
		// User started typing a new command -> Cancel any pending hint (self-correction)
		if pending, exists := e.sessions[key]; exists {
			if pending.timer != nil {
				pending.timer.Stop()
			}
			delete(e.sessions, key)
		}

	case terminal.EventCommandEnd:
		// Check if command succeeded
		if event.ExitCode == 0 {
			// Succeeded -> cancel any pending hint
			if pending, exists := e.sessions[key]; exists {
				if pending.timer != nil {
					pending.timer.Stop()
				}
				delete(e.sessions, key)
			}
			return
		}

		// Command failed -> Start debounce timer
		if pending, exists := e.sessions[key]; exists && pending.timer != nil {
			pending.timer.Stop()
		}

		pending := &pendingSession{
			lastFailed:  event,
			containerID: containerID,
		}

		pending.timer = time.AfterFunc(e.cfg.DebounceDuration, func() {
			e.fireDebouncedHint(userID, sessionID, containerID, event)
		})

		e.sessions[key] = pending
	}
}

// fireDebouncedHint runs the LLM tool loop after the debounce window expires.
func (e *Engine) fireDebouncedHint(userID, sessionID, containerID string, event terminal.ShellEvent) {
	e.mu.Lock()
	key := userID
	delete(e.sessions, key)
	provider := e.llm
	e.mu.Unlock()

	if provider == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	messages := BuildErrorPrompt(event.Command, event.ExitCode, event.PWD, event.Duration.Milliseconds())
	tools := e.registry.GetDefinitions()

	var toolsUsed []string

	// Tool execution loop (max 3 rounds)
	for round := 0; round < 3; round++ {
		resp, err := provider.Generate(ctx, llm.CompletionRequest{
			Messages:    messages,
			Tools:       tools,
			Temperature: 0.3,
		})
		if err != nil {
			slog.Warn("Tutor LLM generation error", "error", err)
			return
		}

		if len(resp.ToolCalls) == 0 {
			// Final text response received
			if e.OnHint != nil && resp.Content != "" {
				e.OnHint(HintEvent{
					Type:      "nudge",
					Content:   resp.Content,
					Command:   event.Command,
					ExitCode:  event.ExitCode,
					ToolsUsed: toolsUsed,
					Timestamp: time.Now(),
					UserID:    userID,
					SessionID: sessionID,
				})
			}
			return
		}

		// Execute tool calls
		messages = append(messages, llm.Message{
			Role:      llm.RoleAssistant,
			ToolCalls: resp.ToolCalls,
		})

		for _, tc := range resp.ToolCalls {
			toolsUsed = append(toolsUsed, tc.Function.Name)
			toolResult, execErr := e.registry.Execute(ctx, containerID, tc.Function.Name, tc.Function.Arguments)
			if execErr != nil {
				toolResult = fmt.Sprintf("Error: %v", execErr)
			}

			if e.OnToolAction != nil {
				e.OnToolAction(tc.Function.Name, tc.Function.Arguments, toolResult)
			}

			messages = append(messages, llm.Message{
				Role:       llm.RoleTool,
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
				Content:    toolResult,
			})
		}
	}
}

// Ask processes direct chat questions from the learner with full terminal command context.
func (e *Engine) Ask(ctx context.Context, userID, sessionID, containerID, question string, history []llm.Message, recentCommands []*domain.CommandLog) (string, []string, error) {
	e.mu.Lock()
	provider := e.llm
	e.mu.Unlock()

	if provider == nil {
		return "AI mentor is currently offline. Please configure an API key or local Ollama model in settings.", nil, nil
	}

	messages := BuildChatPrompt(question, history, recentCommands)
	tools := e.registry.GetDefinitions()
	var toolsUsed []string

	for round := 0; round < 3; round++ {
		resp, err := provider.Generate(ctx, llm.CompletionRequest{
			Messages:    messages,
			Tools:       tools,
			Temperature: 0.5,
		})
		if err != nil {
			return "", nil, fmt.Errorf("generate chat response: %w", err)
		}

		if len(resp.ToolCalls) == 0 {
			return resp.Content, toolsUsed, nil
		}

		messages = append(messages, llm.Message{
			Role:      llm.RoleAssistant,
			ToolCalls: resp.ToolCalls,
		})

		for _, tc := range resp.ToolCalls {
			toolsUsed = append(toolsUsed, tc.Function.Name)
			toolResult, execErr := e.registry.Execute(ctx, containerID, tc.Function.Name, tc.Function.Arguments)
			if execErr != nil {
				toolResult = fmt.Sprintf("Error: %v", execErr)
			}

			messages = append(messages, llm.Message{
				Role:       llm.RoleTool,
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
				Content:    toolResult,
			})
		}
	}

	return "I couldn't finish analyzing the environment. What specific aspect can I help clarify?", toolsUsed, nil
}
