package terminal_test

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/ashureev/shsh-labs/internal/terminal"
)

func b64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func TestTelemetryParser_CompleteEvent(t *testing.T) {
	parser := terminal.NewTelemetryParser()

	var events []terminal.ShellEvent
	parser.OnEvent = func(ev terminal.ShellEvent) {
		events = append(events, ev)
	}

	// 1. Prompt Start
	promptPayload := fmt.Sprintf("\x1b]133;A;pwd=%s\x07", b64("/home/learner/work"))
	parser.Feed([]byte("learner@sandbox:~/work$ " + promptPayload))

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != terminal.EventPromptStart || events[0].PWD != "/home/learner/work" {
		t.Errorf("unexpected prompt event: %+v", events[0])
	}

	// 2. Command Start (PreExec)
	cmd := "ls -la /var/log | grep auth"
	preExecPayload := fmt.Sprintf("\x1b]133;B;cmd=%s;pwd=%s\x07", b64(cmd), b64("/home/learner/work"))
	parser.Feed([]byte(preExecPayload))

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[1].Type != terminal.EventCommandStart || events[1].Command != cmd {
		t.Errorf("unexpected command start event: %+v", events[1])
	}

	// 3. Command Output (Interleaved text)
	parser.Feed([]byte("-rw-r----- 1 syslog adm 1240 auth.log\r\n"))

	// 4. Command End (PostExec with exit code and duration)
	postExecPayload := fmt.Sprintf("\x1b]133;D;exit=0;dur=42;cmd=%s;pwd=%s\x07", b64(cmd), b64("/home/learner/work"))
	parser.Feed([]byte(postExecPayload))

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	lastEvent := events[2]
	if lastEvent.Type != terminal.EventCommandEnd {
		t.Errorf("expected EventCommandEnd, got %v", lastEvent.Type)
	}
	if lastEvent.Command != cmd {
		t.Errorf("expected command %q, got %q", cmd, lastEvent.Command)
	}
	if lastEvent.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", lastEvent.ExitCode)
	}
	if lastEvent.Duration != 42*time.Millisecond {
		t.Errorf("expected duration 42ms, got %v", lastEvent.Duration)
	}
}

func TestTelemetryParser_ChunkedPacketStreaming(t *testing.T) {
	parser := terminal.NewTelemetryParser()

	var events []terminal.ShellEvent
	parser.OnEvent = func(ev terminal.ShellEvent) {
		events = append(events, ev)
	}

	cmd := "chmod 700 /root/.ssh"
	payload := fmt.Sprintf("some output\x1b]133;D;exit=1;dur=15;cmd=%s;pwd=%s\x07prompt$ ", b64(cmd), b64("/root"))

	// Feed character by character to simulate fragmentation across TCP/WebSocket boundaries
	for i := 0; i < len(payload); i++ {
		parser.Feed([]byte{payload[i]})
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.Type != terminal.EventCommandEnd || ev.Command != cmd || ev.ExitCode != 1 || ev.PWD != "/root" {
		t.Errorf("failed chunked stream parsing, got: %+v", ev)
	}
}

func TestTelemetryParser_ComplexQuotesAndSpecialChars(t *testing.T) {
	parser := terminal.NewTelemetryParser()

	var events []terminal.ShellEvent
	parser.OnEvent = func(ev terminal.ShellEvent) {
		events = append(events, ev)
	}

	complexCmd := `python3 -c 'import os; print("hello; world\n$test")' && echo "done > output.txt"`
	payload := fmt.Sprintf("\x1b]133;D;exit=0;dur=100;cmd=%s;pwd=%s\x07", b64(complexCmd), b64("/home/learner"))

	parser.Feed([]byte(payload))

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Command != complexCmd {
		t.Errorf("expected command %q, got %q", complexCmd, events[0].Command)
	}
}

func TestTelemetryParser_StandardOSC133Fallback(t *testing.T) {
	parser := terminal.NewTelemetryParser()

	var events []terminal.ShellEvent
	parser.OnEvent = func(ev terminal.ShellEvent) {
		events = append(events, ev)
	}

	// Standard OSC 133 format without our custom base64 key-values: \x1b]133;D;0\x07
	parser.Feed([]byte("\x1b]133;A\x07"))
	parser.Feed([]byte("\x1b]133;B\x07"))
	parser.Feed([]byte("\x1b]133;D;127\x07"))

	if len(events) != 3 {
		t.Fatalf("expected 3 fallback events, got %d", len(events))
	}
	if events[0].Type != terminal.EventPromptStart {
		t.Errorf("expected prompt start, got %v", events[0].Type)
	}
	if events[1].Type != terminal.EventCommandStart {
		t.Errorf("expected command start, got %v", events[1].Type)
	}
	if events[2].Type != terminal.EventCommandEnd || events[2].ExitCode != 127 {
		t.Errorf("expected exit code 127, got %+v", events[2])
	}
}
