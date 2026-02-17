package terminal

import (
	"testing"
)

func BenchmarkExtractPWDFromOutput(b *testing.B) {
	// Initialize TerminalMonitor with nil dependencies as they are not used in extractPWDFromOutput
	// except for tm.parser which is initialized in NewTerminalMonitor
	tm := NewTerminalMonitor(nil, nil, nil)
	userID := "user1"
	sessionID := "session1"

	// Register session to ensure parser has session info (needed for UpdateCurrentDir)
	tm.RegisterSession(userID, sessionID, "container1", "/home/user")

	data := []byte("some output\ncd /tmp\nmore output")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tm.extractPWDFromOutput(userID, sessionID, data)
	}
}

func BenchmarkDetectPrompt(b *testing.B) {
	tm := NewTerminalMonitor(nil, nil, nil)

	testCases := []string{
		"learner@container:~$ ls -la",
		"[learner@container ~]$ cd /tmp",
		"bash-5.1$ pwd",
		"$ echo hello",
		"# apt-get update",
		"root@server:/var/www$ git status",
		"no match here",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			tm.detectPrompt(tc)
		}
	}
}

func BenchmarkDetectExitCode(b *testing.B) {
	tm := NewTerminalMonitor(nil, nil, nil)

	outputs := []string{
		"command not found",
		"No such file or directory",
		"Permission denied",
		"Invalid argument",
		"Operation not permitted",
		"syntax error",
		"cannot access",
		"not recognized",
		"success output here",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, output := range outputs {
			tm.detectExitCode(output)
		}
	}
}
