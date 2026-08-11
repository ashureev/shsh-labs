package llm

import (
	"strings"
)

// NewProvider constructs the appropriate Provider driver based on configuration.
func NewProvider(cfg ProviderConfig) (Provider, error) {
	providerType := strings.ToLower(strings.TrimSpace(cfg.Provider))
	switch providerType {
	case "gemini":
		return NewGeminiProvider(cfg), nil
	case "openai", "openrouter", "ollama", "":
		return NewOpenAIProvider(cfg), nil
	default:
		// Fall back to OpenAI-compatible endpoint
		return NewOpenAIProvider(cfg), nil
	}
}

// DefaultOllamaConfig returns a default configuration pointing to local Ollama.
func DefaultOllamaConfig(model string) ProviderConfig {
	if model == "" {
		model = "llama3.2"
	}
	return ProviderConfig{
		Provider: "ollama",
		BaseURL:  "http://localhost:11434/v1",
		APIKey:   "ollama",
		Model:    model,
	}
}

// DefaultGeminiConfig returns a default Gemini configuration.
func DefaultGeminiConfig(apiKey string, model string) ProviderConfig {
	if model == "" {
		model = "gemini-2.5-flash"
	}
	return ProviderConfig{
		Provider: "gemini",
		BaseURL:  "https://generativelanguage.googleapis.com",
		APIKey:   apiKey,
		Model:    model,
	}
}
