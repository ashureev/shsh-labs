package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ashureev/shsh-labs/internal/domain"
	"github.com/ashureev/shsh-labs/internal/store"
)

func TestSQLiteStore_CommandsAndChat(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "shsh-store-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	repo, err := store.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()

	// 1. Test Command Logging
	cmdLog := &domain.CommandLog{
		UserID:     "user-1",
		SessionID:  "session-1",
		Command:    "ls -la /var/log",
		PWD:        "/home/learner",
		ExitCode:   0,
		DurationMs: 25,
		CreatedAt:  time.Now(),
	}

	if err := repo.SaveCommand(ctx, cmdLog); err != nil {
		t.Fatalf("save command: %v", err)
	}

	logs, err := repo.GetRecentCommands(ctx, "user-1", "session-1", 10)
	if err != nil {
		t.Fatalf("get recent commands: %v", err)
	}
	if len(logs) != 1 || logs[0].Command != "ls -la /var/log" {
		t.Errorf("unexpected command logs: %+v", logs)
	}

	// 2. Test Chat Messages
	chatMsg := &domain.ChatMessage{
		UserID:    "user-1",
		SessionID: "session-1",
		Role:      "assistant",
		Content:   "Nudge: Check file permissions",
		CreatedAt: time.Now(),
	}

	if err := repo.SaveChatMessage(ctx, chatMsg); err != nil {
		t.Fatalf("save chat message: %v", err)
	}

	messages, err := repo.GetChatHistory(ctx, "user-1", "session-1", 10)
	if err != nil {
		t.Fatalf("get chat history: %v", err)
	}
	if len(messages) != 1 || messages[0].Content != "Nudge: Check file permissions" {
		t.Errorf("unexpected chat messages: %+v", messages)
	}

	// 3. Test Settings
	if err := repo.SetSetting(ctx, "llm_provider", "ollama"); err != nil {
		t.Fatalf("set setting: %v", err)
	}

	val, err := repo.GetSetting(ctx, "llm_provider")
	if err != nil {
		t.Fatalf("get setting: %v", err)
	}
	if val != "ollama" {
		t.Errorf("expected ollama, got %q", val)
	}
}
