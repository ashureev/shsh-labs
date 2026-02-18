// Package agent implements the AI terminal assistant.
package agent

import (
	"time"
)

// ChatRequest represents a chat request to the agent.
type ChatRequest struct {
	Message     string `json:"message"`
	ContainerID string `json:"-"`
	VolumePath  string `json:"-"`
	UserID      string `json:"-"`
	SessionID   string `json:"-"`
}

// ChatResponse represents a chat response from the agent.
type ChatResponse struct {
	Response  string   `json:"response"`
	ToolsUsed []string `json:"tools_used,omitempty"`
}

// SessionEntry represents a command in the session log.
type SessionEntry struct {
	Timestamp  string `json:"ts"`
	Seq        int    `json:"seq"`
	User       string `json:"user"`
	PWD        string `json:"pwd"`
	Command    string `json:"cmd"`
	ExitCode   int    `json:"exit"`
	DurationMs int    `json:"duration_ms"`
}

// ContainerStateOutput represents container inspection results.
type ContainerStateOutput struct {
	Processes   string `json:"processes,omitempty"`
	DiskUsage   string `json:"disk_usage,omitempty"`
	Environment string `json:"environment,omitempty"`
	Uptime      string `json:"uptime,omitempty"`
	WorkingDir  string `json:"working_dir,omitempty"`
}

// ResponseType categorizes agent responses.
type ResponseType string

const (
	// ResponseTypePattern indicates a rule/pattern-based response.
	ResponseTypePattern ResponseType = "pattern"
	// ResponseTypeLLM indicates a model-generated response.
	ResponseTypeLLM ResponseType = "llm"
	// ResponseTypeAlert indicates a high-priority warning.
	ResponseTypeAlert ResponseType = "alert"
	// ResponseTypeSilent indicates an internal response that should not be surfaced.
	ResponseTypeSilent ResponseType = "silent"
	// ResponseTypeError indicates an error response.
	ResponseTypeError ResponseType = "error"
)

// Config holds agent configuration.
type Config struct {
	Provider         string
	ModelName        string
	GoogleAPIKey     string
	OpenRouterAPIKey string
	TypingSpeed      time.Duration
	ThinkPause       time.Duration
	JitterMax        time.Duration
}

// DefaultConfig returns default agent configuration.
func DefaultConfig() Config {
	return Config{
		TypingSpeed: 75 * time.Millisecond,
		ThinkPause:  500 * time.Millisecond,
		JitterMax:   25 * time.Millisecond,
	}
}

// TerminalInput represents a terminal command for agent processing.
type TerminalInput struct {
	Command    string
	PWD        string
	Output     string
	ExitCode   int
	Timestamp  int64
	UserID     string
	SessionID  string
	VolumePath string
	Duration   time.Duration
	HasOSC133  bool
}

// SessionSignalRequest carries transient learner state used by silence policy.
type SessionSignalRequest struct {
	UserID            string
	SessionID         string
	InEditorMode      bool
	IsTyping          bool
	JustSelfCorrected bool
	EditorName        string
	Timestamp         int64
}

// Response represents an agent response.
type Response struct {
	Type           string
	Content        string
	Sidebar        string
	Silent         bool
	Alert          string
	RequireConfirm bool
	Pattern        string
	ToolsUsed      []string
	Block          bool
	UserID         string
	SessionID      string
}
