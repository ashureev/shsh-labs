package agent

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConversationLoggerWritesPerSessionNDJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logger, err := NewConversationLogger(ConversationLogConfig{
		Enabled:   true,
		Dir:       dir,
		QueueSize: 16,
	}, slog.Default())
	if err != nil {
		t.Fatalf("NewConversationLogger failed: %v", err)
	}
	defer func() { _ = logger.Close() }()

	event := ConversationLogEvent{
		UserID:     "user-1",
		SessionID:  "sess-1",
		Channel:    "chat_http",
		Direction:  "outbound",
		EventType:  "chat_user_message",
		ContentRaw: "echo hi",
	}
	logger.Log(event)

	path := filepath.Join(dir, "user-1", "sess-1.ndjson")
	line := waitForLogLine(t, path)
	var got ConversationLogEvent
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("failed to unmarshal log line: %v", err)
	}
	if got.ContentRaw != "echo hi" {
		t.Fatalf("unexpected ContentRaw: %q", got.ContentRaw)
	}
	if got.Content == "" {
		t.Fatal("expected cleaned content to be populated")
	}
}

func TestCleanForReadabilityStripsANSI(t *testing.T) {
	t.Parallel()

	raw := "\x1b[31merror\x1b[0m plain"
	clean := cleanForReadability(raw)
	if strings.Contains(clean, "\x1b[31m") {
		t.Fatalf("expected ANSI sequence to be stripped: %q", clean)
	}
	if !strings.Contains(clean, "error plain") {
		t.Fatalf("expected readable text to remain: %q", clean)
	}
}

func waitForLogLine(t *testing.T, path string) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && len(data) > 0 {
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) > 0 {
				return lines[len(lines)-1]
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for log file %s", path)
	return ""
}
