package agent

import (
	"context"
	"iter"
	"log/slog"
	"time"
)

// Service provides AI chat functionality using the Agent pipeline.
type Service struct {
	processor Processor
}

// NewServiceWithProcessor creates a new agent service with a custom processor.
func NewServiceWithProcessor(processor Processor) (*Service, error) {
	return &Service{
		processor: processor,
	}, nil
}

// Chat processes a user message and returns response chunks.
// This is the main entry point for reactive chat (user-initiated).
func (s *Service) Chat(ctx context.Context, req ChatRequest) iter.Seq2[*ChatResponse, error] {
	return s.processor.Chat(ctx, req)
}

// ProcessTerminalInput processes terminal commands through the agent pipeline.
// This is for proactive assistance (agent-initiated based on terminal activity).
func (s *Service) ProcessTerminalInput(ctx context.Context, input TerminalInput) iter.Seq2[*Response, error] {
	return s.processor.ProcessTerminalInput(ctx, input)
}

// GetStats returns agent statistics.
func (s *Service) GetStats() Stats {
	return s.processor.GetStats()
}

// Stats contains agent statistics.
type Stats struct {
	PatternCount    int `json:"pattern_count"`
	SafetyRuleCount int `json:"safety_rule_count"`
}

// Close releases resources.
func (s *Service) Close() {
	if s.processor != nil {
		s.processor.Close()
	}
}

// UpdateSessionEditorMode updates the editor mode status for a user's session.
// This is called by the terminal monitor when editor mode changes.
func (s *Service) UpdateSessionEditorMode(ctx context.Context, userID, sessionID string, inEditor bool, editorName string) {
	if s.processor == nil {
		return
	}
	if err := s.processor.UpdateSessionSignals(ctx, SessionSignalRequest{
		UserID:       userID,
		SessionID:    sessionID,
		InEditorMode: inEditor,
		EditorName:   editorName,
		Timestamp:    time.Now().Unix(),
	}); err != nil {
		slog.Warn("failed to update editor mode signal", "user_id", userID, "session_id", sessionID, "error", err)
	}
}

// UpdateSessionTypingStatus updates typing signal in Python session state.
func (s *Service) UpdateSessionTypingStatus(ctx context.Context, userID, sessionID string, isTyping bool) {
	if s.processor == nil {
		return
	}
	if err := s.processor.UpdateSessionSignals(ctx, SessionSignalRequest{
		UserID:    userID,
		SessionID: sessionID,
		IsTyping:  isTyping,
		Timestamp: time.Now().Unix(),
	}); err != nil {
		slog.Warn("failed to update typing signal", "user_id", userID, "session_id", sessionID, "error", err)
	}
}

// UpdateSessionSelfCorrected updates self-corrected signal in Python session state.
func (s *Service) UpdateSessionSelfCorrected(ctx context.Context, userID, sessionID string, selfCorrected bool) {
	if s.processor == nil {
		return
	}
	if err := s.processor.UpdateSessionSignals(ctx, SessionSignalRequest{
		UserID:            userID,
		SessionID:         sessionID,
		JustSelfCorrected: selfCorrected,
		Timestamp:         time.Now().Unix(),
	}); err != nil {
		slog.Warn("failed to update self-corrected signal", "user_id", userID, "session_id", sessionID, "error", err)
	}
}
