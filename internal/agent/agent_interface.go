package agent

import (
	"context"
	"iter"
)

// AgentProcessor defines the interface for AI agent processing.
// This interface is implemented by the gRPC client.
//
//nolint:revive // Stutter kept for public API stability.
type AgentProcessor interface {
	// ProcessTerminalInput processes terminal commands through the AI pipeline
	ProcessTerminalInput(ctx context.Context, input TerminalInput) iter.Seq2[*Response, error]

	// Chat processes a user message and returns response chunks
	Chat(ctx context.Context, req ChatRequest) iter.Seq2[*ChatResponse, error]

	// UpdateSessionSignals syncs typing/editor/self-correct flags with Python session state
	UpdateSessionSignals(ctx context.Context, req SessionSignalRequest) error

	// GetStats returns agent statistics
	GetStats() AgentStats

	// Close releases resources
	Close()
}

// Ensure GrpcClient implements AgentProcessor.
var _ AgentProcessor = (*GrpcClient)(nil)
