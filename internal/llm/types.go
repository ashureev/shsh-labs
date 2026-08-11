package llm

import (
	"context"
	"time"
)

// Role represents a message author role.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// FunctionCall describes the function invocation requested by the model.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolCall represents a specific tool invocation request from the LLM.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// Message represents a single turn in a conversation.
type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// ToolDefinition defines a callable function tool available to the LLM.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// CompletionRequest is a unified request format for any LLM provider.
type CompletionRequest struct {
	Model       string           `json:"model,omitempty"`
	Messages    []Message        `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	Temperature float32          `json:"temperature,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
}

// Usage captures token consumption metrics.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// CompletionResponse is the unified result from an LLM call.
type CompletionResponse struct {
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	FinishReason string     `json:"finish_reason"`
	Usage        Usage      `json:"usage"`
}

// StreamChunk represents an incremental token or event in a streaming LLM response.
type StreamChunk struct {
	DeltaContent string    `json:"delta_content,omitempty"`
	ToolCall     *ToolCall `json:"tool_call,omitempty"`
	FinishReason string    `json:"finish_reason,omitempty"`
	Error        error     `json:"error,omitempty"`
}

// ProviderConfig holds credentials and endpoints for a provider.
type ProviderConfig struct {
	Provider string        `json:"provider"` // "openai", "gemini", "anthropic", "ollama", "openrouter"
	BaseURL  string        `json:"base_url"`
	APIKey   string        `json:"api_key"`
	Model    string        `json:"model"`
	Timeout  time.Duration `json:"timeout"`
}

// Provider is the common interface implemented by all LLM drivers.
type Provider interface {
	Generate(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
	Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
}
