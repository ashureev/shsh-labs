// Package terminal provides terminal WebSocket handling and PTY control.
package terminal

import (
	"log/slog"
	"sync"
	"time"

	"github.com/docker/docker/client"
)

// PTYController provides AI typing capabilities for terminal sessions.
// It can inject keystrokes into a Docker container's terminal to simulate
// human-like typing for demonstrations.
type PTYController struct {
	dockerClient *client.Client
	config       PTYConfig
	mu           sync.RWMutex
	logger       *slog.Logger
}

// PTYConfig holds configuration for PTY operations.
type PTYConfig struct {
	// TypingSpeed is the base delay between keystrokes (default: 75ms)
	TypingSpeed time.Duration
	// JitterMax is the maximum random jitter added to typing speed (default: 25ms)
	JitterMax time.Duration
	// ThinkPause is the delay before typing the first character (default: 500ms)
	ThinkPause time.Duration
	// PunctuationPause is extra delay after punctuation (default: 100ms)
	PunctuationPause time.Duration
}

// DefaultPTYConfig returns sensible defaults for human-like typing.
func DefaultPTYConfig() PTYConfig {
	return PTYConfig{
		TypingSpeed:      75 * time.Millisecond,
		JitterMax:        25 * time.Millisecond,
		ThinkPause:       500 * time.Millisecond,
		PunctuationPause: 100 * time.Millisecond,
	}
}

// TypeResult contains information about a typing operation.
type TypeResult struct {
	Command         string
	CharactersTyped int
	Duration        time.Duration
	Executed        bool
	Error           error
}

// NewPTYController creates a new PTY controller for AI typing.
func NewPTYController(dockerClient *client.Client, config PTYConfig, logger *slog.Logger) *PTYController {
	if logger == nil {
		logger = slog.Default()
	}
	return &PTYController{
		dockerClient: dockerClient,
		config:       config,
		logger:       logger,
	}
}

// SetTypingSpeed updates the typing speed configuration.
func (p *PTYController) SetTypingSpeed(speed time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config.TypingSpeed = speed
}

// GetConfig returns the current PTY configuration.
func (p *PTYController) GetConfig() PTYConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}
