package terminal

import (
	"testing"
	"time"
)

func TestOSC133MarkerExtraction(t *testing.T) {
	parser := NewOSC133CommandParser(nil)

	tests := []struct {
		name      string
		input     []byte
		wantType  string
		wantData  string
		wantFound bool
	}{
		{
			name:      "Prompt start marker",
			input:     []byte{0x1b, ']', '1', '3', '3', ';', 'A', 0x07},
			wantType:  OSC133PromptStart,
			wantData:  "",
			wantFound: true,
		},
		{
			name:      "Pre-exec marker",
			input:     []byte{0x1b, ']', '1', '3', '3', ';', 'B', 0x07},
			wantType:  OSC133PreExec,
			wantData:  "",
			wantFound: true,
		},
		{
			name:      "Command exec marker",
			input:     []byte{0x1b, ']', '1', '3', '3', ';', 'C', 0x07},
			wantType:  OSC133CommandExec,
			wantData:  "",
			wantFound: true,
		},
		{
			name:      "Command exit marker with exit code 0",
			input:     []byte{0x1b, ']', '1', '3', '3', ';', 'D', ';', '0', 0x07},
			wantType:  OSC133CommandExit,
			wantData:  "0",
			wantFound: true,
		},
		{
			name:      "Command exit marker with exit code 1",
			input:     []byte{0x1b, ']', '1', '3', '3', ';', 'D', ';', '1', 0x07},
			wantType:  OSC133CommandExit,
			wantData:  "1",
			wantFound: true,
		},
		{
			name:      "No OSC 133 marker",
			input:     []byte("Hello World"),
			wantType:  "",
			wantData:  "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.extractOSC133Marker(tt.input)
			if tt.wantFound {
				if got == nil {
					t.Errorf("expected marker but got nil")
					return
				}
				if got.Type != tt.wantType {
					t.Errorf("marker type = %v, want %v", got.Type, tt.wantType)
				}
				if got.Data != tt.wantData {
					t.Errorf("marker data = %v, want %v", got.Data, tt.wantData)
				}
			} else {
				if got != nil {
					t.Errorf("expected no marker but got %+v", got)
				}
			}
		})
	}
}

func TestOSC133CommandLifecycle(t *testing.T) {
	parser := NewOSC133CommandParser(nil)
	userID := "test-user"
	containerID := "test-container"

	parser.RegisterSession(userID, containerID)
	defer parser.UnregisterSession(userID)

	command := "ls -la"
	parser.SetCommandBuffer(userID, command)

	preExecMarker := parser.extractOSC133Marker([]byte{0x1b, ']', '1', '3', '3', ';', 'B', 0x07})
	entry := parser.handleOSC133Marker(userID, preExecMarker)
	if entry != nil {
		t.Errorf("pre-exec should not produce command entry, got %+v", entry)
	}

	execMarker := parser.extractOSC133Marker([]byte{0x1b, ']', '1', '3', '3', ';', 'C', 0x07})
	entry = parser.handleOSC133Marker(userID, execMarker)
	if entry != nil {
		t.Errorf("exec marker should not produce entry, got %+v", entry)
	}

	time.Sleep(10 * time.Millisecond)
	exitMarker := parser.extractOSC133Marker([]byte{0x1b, ']', '1', '3', '3', ';', 'D', ';', '0', 0x07})
	entry = parser.handleOSC133Marker(userID, exitMarker)
	if entry == nil {
		t.Error("exit marker should produce command entry")
		return
	}

	if entry.Command != command {
		t.Errorf("entry.Command = %v, want %v", entry.Command, command)
	}

	if entry.ExitCode != 0 {
		t.Errorf("entry.ExitCode = %v, want %v", entry.ExitCode, 0)
	}

	if entry.Duration <= 0 {
		t.Errorf("entry.Duration = %v, want > 0", entry.Duration)
	}

	if entry.Sequence != 1 {
		t.Errorf("entry.Sequence = %v, want %v", entry.Sequence, 1)
	}

	if !parser.HasOSC133Support(userID) {
		t.Error("OSC 133 support should be detected")
	}
}

func TestOSC133FallbackInputProcessing(t *testing.T) {
	parser := NewOSC133CommandParser(nil)
	userID := "test-user"

	parser.RegisterSession(userID, "test-container")
	defer parser.UnregisterSession(userID)

	_, executed := parser.ProcessInput(userID, []byte("l"))
	if executed {
		t.Error("partial command should not execute")
	}

	_, executed = parser.ProcessInput(userID, []byte("s"))
	if executed {
		t.Error("partial command should not execute")
	}

	cmd, executed := parser.ProcessInput(userID, []byte("\n"))
	if !executed {
		t.Error("Enter should execute command")
	}

	if cmd != "ls" {
		t.Errorf("cmd = %v, want ls", cmd)
	}

	if parser.HasOSC133Support(userID) {
		t.Error("OSC 133 support should not be detected without markers")
	}
}

func TestOSC133FallbackIgnoresArrowEscapeSequences(t *testing.T) {
	parser := NewOSC133CommandParser(nil)
	userID := "test-user-escape"

	parser.RegisterSession(userID, "test-container")
	defer parser.UnregisterSession(userID)

	// Up arrow key (ESC [ A) should not be treated as literal command text.
	if cmd, executed := parser.ProcessInput(userID, []byte{0x1b, '[', 'A'}); executed || cmd != "" {
		t.Fatalf("escape sequence should not execute command, got executed=%v cmd=%q", executed, cmd)
	}

	// Next real command should execute correctly and not include leftover "[A".
	_, _ = parser.ProcessInput(userID, []byte("docker"))
	cmd, executed := parser.ProcessInput(userID, []byte("\n"))
	if !executed {
		t.Fatal("expected enter to execute command")
	}
	if cmd != "docker" {
		t.Fatalf("cmd = %q, want %q", cmd, "docker")
	}
}

func BenchmarkOSC133MarkerExtraction(b *testing.B) {
	parser := NewOSC133CommandParser(nil)

	data := []byte{0x1b, ']', '1', '3', '3', ';', 'A', 0x07}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.extractOSC133Marker(data)
	}
}
