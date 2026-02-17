package terminal

import (
	"log/slog"
	"os"
	"testing"
)

func TestOSC133EditorMarkers(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	parser := NewOSC133CommandParser(logger)

	// Register a session
	parser.RegisterSession("test-user", "container-123")

	// Test initial state
	if parser.IsInEditor("test-user") {
		t.Error("Expected not in editor mode initially")
	}
	if parser.GetEditorName("test-user") != "" {
		t.Error("Expected empty editor name initially")
	}

	// Test editor start marker for vim
	t.Run("EditorStart_vim", func(t *testing.T) {
		data := []byte("\x1b]133;G;vim\x07")
		parser.ProcessOutput("test-user", data)

		if !parser.IsInEditor("test-user") {
			t.Error("Expected in editor mode after vim start marker")
		}
		if parser.GetEditorName("test-user") != "vim" {
			t.Errorf("Expected editor name 'vim', got '%s'", parser.GetEditorName("test-user"))
		}
	})

	// Test editor end marker
	t.Run("EditorEnd", func(t *testing.T) {
		data := []byte("\x1b]133;H\x07")
		parser.ProcessOutput("test-user", data)

		if parser.IsInEditor("test-user") {
			t.Error("Expected not in editor mode after end marker")
		}
		if parser.GetEditorName("test-user") != "" {
			t.Errorf("Expected empty editor name, got '%s'", parser.GetEditorName("test-user"))
		}
	})

	// Test nano editor
	t.Run("EditorStart_nano", func(t *testing.T) {
		data := []byte("\x1b]133;G;nano\x07")
		parser.ProcessOutput("test-user", data)

		if !parser.IsInEditor("test-user") {
			t.Error("Expected in editor mode after nano start marker")
		}
		if parser.GetEditorName("test-user") != "nano" {
			t.Errorf("Expected editor name 'nano', got '%s'", parser.GetEditorName("test-user"))
		}
	})

	// Test non-existent session
	t.Run("NonExistentSession", func(t *testing.T) {
		if parser.IsInEditor("non-existent") {
			t.Error("Expected not in editor mode for non-existent session")
		}
		if parser.GetEditorName("non-existent") != "" {
			t.Error("Expected empty editor name for non-existent session")
		}
	})
}
