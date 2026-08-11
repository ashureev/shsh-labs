package domain

import "time"

// CommandLog represents a recorded shell command execution in the database.
type CommandLog struct {
	ID         int64     `json:"id"`
	UserID     string    `json:"user_id"`
	SessionID  string    `json:"session_id"`
	Command    string    `json:"command"`
	PWD        string    `json:"pwd"`
	ExitCode   int       `json:"exit_code"`
	DurationMs int64     `json:"duration_ms"`
	CreatedAt  time.Time `json:"created_at"`
}

// ChatMessage represents a stored conversation message between user and AI mentor.
type ChatMessage struct {
	ID        int64     `json:"id"`
	UserID    string    `json:"user_id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	ToolsJSON string    `json:"tools_json,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// UserSettings represents configured LLM providers and tutor preferences.
type UserSettings struct {
	Provider       string `json:"provider"`
	Model          string `json:"model"`
	APIKey         string `json:"api_key,omitempty"`
	BaseURL        string `json:"base_url,omitempty"`
	EnableAmbient  bool   `json:"enable_ambient"`
	TutorVerbosity string `json:"tutor_verbosity"` // "scaffold", "detailed", "minimal"
}
