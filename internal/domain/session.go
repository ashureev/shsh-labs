package domain

import (
	"time"
)

// CommandEntry represents a single command in the session history.
type CommandEntry struct {
	Command   string
	Timestamp time.Time
	ExitCode  int
	Output    string
}

// LearnerSession holds session state for a learner.
type LearnerSession struct {
	UserID              string
	ContainerID         string
	CurrentChallenge    *Challenge
	AttemptCount        int
	JustSelfCorrected   bool
	LastCommand         string
	LastProactiveMsg    time.Time
	IsTyping            bool
	CommandHistory      []CommandEntry
	ErrorCount          int
	SelfCorrectionCount int
}

// RecordCommand adds a command to the session history.
func (s *LearnerSession) RecordCommand(cmd string, exitCode int, output string) {
	s.CommandHistory = append(s.CommandHistory, CommandEntry{
		Command:   cmd,
		Timestamp: time.Now(),
		ExitCode:  exitCode,
		Output:    output,
	})
	s.LastCommand = cmd

	if exitCode != 0 {
		s.ErrorCount++
	}
}

// RecentCommands returns the last n commands from history.
func (s *LearnerSession) RecentCommands(n int) []CommandEntry {
	if n >= len(s.CommandHistory) {
		return s.CommandHistory
	}
	return s.CommandHistory[len(s.CommandHistory)-n:]
}
