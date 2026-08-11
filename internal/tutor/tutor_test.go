package tutor_test

import (
	"context"
	"testing"
	"time"

	"github.com/ashureev/shsh-labs/internal/domain"
	"github.com/ashureev/shsh-labs/internal/llm"
	"github.com/ashureev/shsh-labs/internal/terminal"
	"github.com/ashureev/shsh-labs/internal/tutor"
)

// MockContainerManager implements tutor.ContainerRunner.
type mockContainerRunner struct {
	execHandler func(cmd []string) (string, int, error)
}

func (m *mockContainerRunner) ExecCommand(ctx context.Context, containerID string, cmd []string) (string, int, error) {
	if m.execHandler != nil {
		return m.execHandler(cmd)
	}
	return "", 0, nil
}

// MockLLMProvider implements llm.Provider.
type mockLLMProvider struct {
	generateHandler func(req llm.CompletionRequest) (*llm.CompletionResponse, error)
}

func (m *mockLLMProvider) Generate(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	if m.generateHandler != nil {
		return m.generateHandler(req)
	}
	return &llm.CompletionResponse{Content: "Default tutor hint"}, nil
}

func (m *mockLLMProvider) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 1)
	go func() {
		defer close(ch)
		ch <- llm.StreamChunk{DeltaContent: "Streaming hint"}
	}()
	return ch, nil
}

func TestToolRegistry_ExecuteTool(t *testing.T) {
	mockRunner := &mockContainerRunner{
		execHandler: func(cmd []string) (string, int, error) {
			if len(cmd) >= 2 && cmd[0] == "ls" {
				return "file1.txt\nfile2.txt", 0, nil
			}
			return "", 0, nil
		},
	}

	registry := tutor.NewToolRegistry(mockRunner)

	output, err := registry.Execute(context.Background(), "c123", "list_directory", `{"path":"/home/learner"}`)
	if err != nil {
		t.Fatalf("unexpected error executing tool: %v", err)
	}

	if output != "file1.txt\nfile2.txt" {
		t.Errorf("unexpected output: %q", output)
	}
}

func TestTutorEngine_DebounceAndSelfCorrection(t *testing.T) {
	llmCalled := false
	mockLLM := &mockLLMProvider{
		generateHandler: func(req llm.CompletionRequest) (*llm.CompletionResponse, error) {
			llmCalled = true
			return &llm.CompletionResponse{Content: "Nudge: Check file path"}, nil
		},
	}

	mockRunner := &mockContainerRunner{}
	engine := tutor.NewEngine(mockLLM, mockRunner, tutor.Config{
		DebounceDuration: 50 * time.Millisecond,
	})

	var emittedHints []tutor.HintEvent
	engine.OnHint = func(h tutor.HintEvent) {
		emittedHints = append(emittedHints, h)
	}

	// 1. User runs a failing command
	engine.HandleShellEvent(terminal.ShellEvent{
		Type:      terminal.EventCommandEnd,
		Command:   "cat /etc/shdow",
		ExitCode:  1,
		PWD:       "/home/learner",
		Timestamp: time.Now(),
	}, "user-1", "session-1", "container-1")

	// 2. User quickly self-corrects within 20ms (< 50ms debounce)
	time.Sleep(20 * time.Millisecond)
	engine.HandleShellEvent(terminal.ShellEvent{
		Type:      terminal.EventCommandStart,
		Command:   "cat /etc/shadow",
		PWD:       "/home/learner",
		Timestamp: time.Now(),
	}, "user-1", "session-1", "container-1")

	// Wait for debounce window to pass
	time.Sleep(60 * time.Millisecond)

	// Since user self-corrected, no hint should be emitted!
	if len(emittedHints) != 0 || llmCalled {
		t.Errorf("expected 0 hints due to self-correction, got %d hints, llmCalled=%v", len(emittedHints), llmCalled)
	}
}

func TestTutorEngine_DebounceFiresOnStuck(t *testing.T) {
	mockLLM := &mockLLMProvider{
		generateHandler: func(req llm.CompletionRequest) (*llm.CompletionResponse, error) {
			return &llm.CompletionResponse{Content: "Notice the spelling in /etc/shdow"}, nil
		},
	}

	mockRunner := &mockContainerRunner{}
	engine := tutor.NewEngine(mockLLM, mockRunner, tutor.Config{
		DebounceDuration: 30 * time.Millisecond,
	})

	hintChan := make(chan tutor.HintEvent, 1)
	engine.OnHint = func(h tutor.HintEvent) {
		hintChan <- h
	}

	// User runs failing command and does nothing (is stuck)
	engine.HandleShellEvent(terminal.ShellEvent{
		Type:      terminal.EventCommandEnd,
		Command:   "cat /etc/shdow",
		ExitCode:  1,
		PWD:       "/home/learner",
		Timestamp: time.Now(),
	}, "user-1", "session-1", "container-1")

	select {
	case h := <-hintChan:
		if h.Content != "Notice the spelling in /etc/shdow" {
			t.Errorf("unexpected hint content: %q", h.Content)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for hint debounce")
	}
}

func TestTutorEngine_AskWithCommandHistory(t *testing.T) {
	var receivedPrompt string
	mockLLM := &mockLLMProvider{
		generateHandler: func(req llm.CompletionRequest) (*llm.CompletionResponse, error) {
			if len(req.Messages) > 0 {
				receivedPrompt = req.Messages[0].Content
			}
			return &llm.CompletionResponse{Content: "You previously ran ls -la in /home/learner"}, nil
		},
	}

	mockRunner := &mockContainerRunner{}
	engine := tutor.NewEngine(mockLLM, mockRunner, tutor.DefaultConfig())

	commands := []*domain.CommandLog{
		{
			Command:    "ls -la",
			PWD:        "/home/learner",
			ExitCode:   0,
			DurationMs: 15,
		},
	}

	ans, tools, err := engine.Ask(context.Background(), "u1", "s1", "c1", "What was my last command?", nil, commands)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ans != "You previously ran ls -la in /home/learner" {
		t.Errorf("unexpected answer: %q", ans)
	}

	if len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(tools))
	}

	if receivedPrompt == "" {
		t.Errorf("expected receivedPrompt to not be empty")
	}
}
