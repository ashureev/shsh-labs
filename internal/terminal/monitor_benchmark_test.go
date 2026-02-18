package terminal

import (
	"testing"
)

func BenchmarkExtractPWDFromOutput(b *testing.B) {
	// Initialize TerminalMonitor with nil dependencies as they are not used in extractPWDFromOutput
	// except for tm.parser which is initialized in NewTerminalMonitor
	tm := NewMonitor(nil, nil, nil)
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
	tm := NewMonitor(nil, nil, nil)

	// Use []byte inputs to match the production detectPromptBytes path.
	testCases := [][]byte{
		[]byte("learner@container:~$ ls -la"),
		[]byte("[learner@container ~]$ cd /tmp"),
		[]byte("bash-5.1$ pwd"),
		[]byte("$ echo hello"),
		[]byte("# apt-get update"),
		[]byte("root@server:/var/www$ git status"),
		[]byte("no match here"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			tm.detectPromptBytes(tc)
		}
	}
}

func BenchmarkDetectExitCode(b *testing.B) {
	tm := NewMonitor(nil, nil, nil)

	// Use []byte inputs to match the production detectExitCodeBytes path.
	outputs := [][]byte{
		[]byte("command not found"),
		[]byte("No such file or directory"),
		[]byte("Permission denied"),
		[]byte("Invalid argument"),
		[]byte("Operation not permitted"),
		[]byte("syntax error"),
		[]byte("cannot access"),
		[]byte("not recognized"),
		[]byte("success output here"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, output := range outputs {
			tm.detectExitCodeBytes(output)
		}
	}
}
