package agent

import (
	"context"
	"iter"
)

// Processor defines the interface for AI agent processing.
// This interface is implemented by the gRPC client.
type Processor interface {
	// ProcessTerminalInput processes terminal commands through the AI pipeline
	ProcessTerminalInput(ctx context.Context, input TerminalInput) iter.Seq2[*Response, error]

	// Chat processes a user message and returns response chunks
	Chat(ctx context.Context, req ChatRequest) iter.Seq2[*ChatResponse, error]

	// UpdateSessionSignals syncs typing/editor/self-correct flags with Python session state
	UpdateSessionSignals(ctx context.Context, req SessionSignalRequest) error

	// ResetSession clears chat/checkpoint/session state for a specific tab session.
	ResetSession(ctx context.Context, userID, sessionID string) error

	// GetStats returns agent statistics
	GetStats() Stats

	// Close releases resources
	Close()
}

// Ensure GrpcClient implements Processor.
var _ Processor = (*GrpcClient)(nil)
