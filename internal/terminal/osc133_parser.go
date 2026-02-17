package terminal

import (
	"bytes"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"
)

// OSC 133 marker types
const (
	OSC133PromptStart  = "A" // Prompt start
	OSC133PreExec      = "B" // Pre-execution (command start)
	OSC133CommandStart = "B" // Alias for B
	OSC133CommandExec  = "C" // Execution start
	OSC133CommandExit  = "D" // Command finished with exit code
	OSC133PreExecAlt   = "E" // Pre-execution (alternate)
	OSC133PostExec     = "F" // Post-execution (alternate)

	// Editor marker types (custom extension)
	OSC133EditorStart = "G" // Editor started (custom marker)
	OSC133EditorEnd   = "H" // Editor exited (custom marker)
)

// MaxCommandHistory is the maximum number of commands to keep in history.
// When exceeded, oldest entries are removed in batches to avoid frequent reslicing.
const MaxCommandHistory = 1000

// CommandHistoryBatchSize is the number of entries to remove when history limit is exceeded.
// This batch removal prevents frequent reslicing operations.
const CommandHistoryBatchSize = 100

// OSC133Marker represents a parsed OSC 133 marker
type OSC133Marker struct {
	Type      string // A, B, C, D, E, or F
	Data      string // Optional data (e.g., exit code for type D)
	Timestamp time.Time
}

// OSC133Session tracks command state for a user session using OSC 133 markers
type OSC133Session struct {
	UserID         string
	ContainerID    string
	CurrentCommand strings.Builder
	LastCommand    string
	PendingCommand string // Command captured from input, waiting for OSC 133 markers
	ExitCode       int
	CommandStart   time.Time
	CommandEnd     time.Time
	CurrentDir     string
	HasOSC133      bool           // Whether OSC 133 markers have been detected
	State          OSC133State    // Current state machine state
	ExpectedSeq    int            // Sequence number for markers
	CommandHistory []CommandEntry // History of executed commands
	InEditor       bool           // Whether user is currently in an editor
	EditorName     string         // Name of active editor (vim, nano, etc.)
	InEscapeSeq    bool           // Whether ANSI escape sequence parsing is in progress
	EscSawBracket  bool           // Whether ESC [ has been seen for CSI sequence
}

// OSC133State represents the state machine for OSC 133 processing
type OSC133State int

const (
	OSC133StateIdle      OSC133State = iota // Waiting for prompt
	OSC133StateInPrompt                     // In prompt, waiting for command
	OSC133StateExecuting                    // Command executing
	OSC133StateCompleted                    // Command completed, waiting for next prompt
)

// CommandEntry represents a completed command with metadata
type CommandEntry struct {
	Sequence  int
	Command   string
	PWD       string
	ExitCode  int
	Duration  time.Duration
	Timestamp time.Time
	StartTime time.Time
	EndTime   time.Time
}

// OSC133CommandParser parses OSC 133 markers from terminal output
type OSC133CommandParser struct {
	sessions map[string]*OSC133Session
	mu       sync.RWMutex
	logger   *slog.Logger

	// OSC 133 marker regex patterns
	promptStartRegex *regexp.Regexp
	preExecRegex     *regexp.Regexp
	commandExecRegex *regexp.Regexp
	commandExitRegex *regexp.Regexp
	postExecRegex    *regexp.Regexp

	// Fallback prompt detection (when OSC 133 not available)
	promptRegex *regexp.Regexp

	// Editor detection patterns
	editorStartRegex *regexp.Regexp
	editorEndRegex   *regexp.Regexp
}

// NewOSC133CommandParser creates a new OSC 133 command parser
func NewOSC133CommandParser(logger *slog.Logger) *OSC133CommandParser {
	if logger == nil {
		logger = slog.Default()
	}

	return &OSC133CommandParser{
		sessions: make(map[string]*OSC133Session),
		logger:   logger,

		// OSC 133 marker patterns: \x1b]133;<type>;<data>\x07
		promptStartRegex: regexp.MustCompile(`\x1b\]133;A(?:;[^\x07]*)?\x07`),
		preExecRegex:     regexp.MustCompile(`\x1b\]133;B(?:;[^\x07]*)?\x07`),
		commandExecRegex: regexp.MustCompile(`\x1b\]133;C(?:;[^\x07]*)?\x07`),
		commandExitRegex: regexp.MustCompile(`\x1b\]133;D;(\d+)?(?:;[^\x07]*)?\x07`),
		postExecRegex:    regexp.MustCompile(`\x1b\]133;F(?:;[^\x07]*)?\x07`),

		// Fallback prompt pattern
		promptRegex: regexp.MustCompile(`(?:\[?\w+@[\w-]+[:~]?[^\]]*\]?\$\s*|\w+:\s*[^\$]*\$\s*|bash-[\d.]+\$\s*|\$\s*|#\s*|>\s*)`),

		// Editor detection patterns
		editorStartRegex: regexp.MustCompile(`\x1b\]133;G;([^\x07]*)\x07`),
		editorEndRegex:   regexp.MustCompile(`\x1b\]133;H(?:;[^\x07]*)?\x07`),
	}
}

// RegisterSession registers a new session for OSC 133 tracking
func (p *OSC133CommandParser) RegisterSession(userID, containerID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.sessions[userID] = &OSC133Session{
		UserID:         userID,
		ContainerID:    containerID,
		HasOSC133:      false,
		State:          OSC133StateIdle,
		CommandHistory: make([]CommandEntry, 0, 100),
	}

	p.logger.Info("[OSC133] Session registered", "user_id", userID, "container_id", containerID)
}

// UnregisterSession removes a session from tracking
func (p *OSC133CommandParser) UnregisterSession(userID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.sessions, userID)
	p.logger.Info("[OSC133] Session unregistered", "user_id", userID)
}

// ProcessOutput processes terminal output and detects OSC 133 markers
// Returns the completed command if one is detected
func (p *OSC133CommandParser) ProcessOutput(userID string, data []byte) *CommandEntry {
	previewLen := len(data)
	if previewLen > 50 {
		previewLen = 50
	}
	p.logger.Info("[OSC133] Processing output",
		"user_id", userID,
		"data_len", len(data),
		"data_preview", string(data[:previewLen]),
	)

	// Try to detect OSC 133 markers first - process ALL markers in order
	markers := p.extractAllOSC133Markers(data)
	p.logger.Info("[OSC133] Markers found", "user_id", userID, "count", len(markers))
	if len(markers) > 0 {
		var finalEntry *CommandEntry
		for _, marker := range markers {
			p.logger.Info("[OSC133] Processing marker", "user_id", userID, "marker_type", marker.Type, "marker_data", marker.Data)
			if entry := p.handleOSC133Marker(userID, marker); entry != nil {
				finalEntry = entry // Keep the last completed command
			}
		}
		return finalEntry
	}

	// Fallback to prompt detection if no OSC 133 markers found
	p.mu.Lock()
	session := p.sessions[userID]
	p.mu.Unlock()

	if session == nil {
		return nil
	}

	// Check for fallback prompt
	if p.promptRegex.Match(data) {
		p.logger.Debug("[OSC133] Fallback prompt detected", "user_id", userID)
		return nil
	}

	return nil
}

// ProcessInput processes raw keyboard input (fallback for shells without OSC 133)
func (p *OSC133CommandParser) ProcessInput(userID string, data []byte) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	session := p.sessions[userID]
	if session == nil {
		return "", false
	}

	// Skip input processing when in editor mode
	if session.InEditor {
		return "", false
	}

	// Process keystrokes for command detection
	for _, b := range data {
		// Track and ignore ANSI escape/control sequences (e.g. arrow keys: ESC [ A).
		if session.InEscapeSeq {
			if !session.EscSawBracket {
				if b == '[' {
					session.EscSawBracket = true
					continue
				}
				// Non-CSI escape sequence, stop swallowing after one byte.
				session.InEscapeSeq = false
				session.EscSawBracket = false
				continue
			}
			// End of CSI sequence is in the 0x40-0x7E range.
			if b >= 0x40 && b <= 0x7E {
				session.InEscapeSeq = false
				session.EscSawBracket = false
			}
			continue
		}

		switch b {
		case 0x1b:
			session.InEscapeSeq = true
			session.EscSawBracket = false
			continue
		case '\r', '\n':
			cmd := session.CurrentCommand.String()
			if cmd != "" {
				session.LastCommand = cmd
				session.PendingCommand = cmd // Store for OSC 133 handler
				session.CurrentCommand.Reset()
				session.CommandStart = time.Now()
				p.logger.Info("[OSC133] Command captured from input", "user_id", userID, "command", cmd)
				return cmd, true
			}
			session.CurrentCommand.Reset()

		case 0x7f, 0x08:
			current := session.CurrentCommand.String()
			if len(current) > 0 {
				runes := []rune(current)
				if len(runes) > 0 {
					session.CurrentCommand.Reset()
					session.CurrentCommand.WriteString(string(runes[:len(runes)-1]))
				}
			}

		default:
			if b >= 0x20 {
				session.CurrentCommand.WriteByte(b)
				p.logger.Debug("[OSC133] Char added to buffer", "user_id", userID, "char", string(b), "buffer", session.CurrentCommand.String())
			}
		}
	}

	return "", false
}

// extractOSC133Marker extracts an OSC 133 marker from data if present
func (p *OSC133CommandParser) extractOSC133Marker(data []byte) *OSC133Marker {
	markers := []struct {
		regex *regexp.Regexp
		typ   string
	}{
		{p.promptStartRegex, OSC133PromptStart},
		{p.preExecRegex, OSC133PreExec},
		{p.commandExecRegex, OSC133CommandExec},
		{p.commandExitRegex, OSC133CommandExit},
		{p.postExecRegex, OSC133PostExec},
		{p.editorStartRegex, OSC133EditorStart},
		{p.editorEndRegex, OSC133EditorEnd},
	}

	for _, m := range markers {
		if matches := m.regex.FindSubmatch(data); len(matches) > 0 {
			marker := &OSC133Marker{
				Type:      m.typ,
				Timestamp: time.Now(),
			}
			if len(matches) > 1 {
				marker.Data = string(matches[1])
			}
			return marker
		}
	}

	return nil
}

// extractAllOSC133Markers extracts all OSC 133 markers from data in order
func (p *OSC133CommandParser) extractAllOSC133Markers(data []byte) []*OSC133Marker {
	var markers []*OSC133Marker
	remaining := data

	for len(remaining) > 0 {
		// Find the next ESC sequence
		escPos := bytes.IndexByte(remaining, 0x1b)
		if escPos == -1 {
			break
		}

		// Check if this is an OSC 133 sequence
		if escPos+6 < len(remaining) && remaining[escPos+1] == ']' && bytes.HasPrefix(remaining[escPos+2:], []byte("133;")) {
			// Find the ST (BEL: 0x07 or ESC: 0x1b 0x5c)
			stPos := escPos + 6
			for stPos < len(remaining) && remaining[stPos] != 0x07 {
				stPos++
			}

			if stPos < len(remaining) {
				// Extract marker
				markerData := remaining[escPos : stPos+1]
				if marker := p.extractOSC133Marker(markerData); marker != nil {
					markers = append(markers, marker)
				}
				remaining = remaining[stPos+1:]
				continue
			}
		}

		remaining = remaining[escPos+1:]
	}

	return markers
}

// handleOSC133Marker processes an OSC 133 marker and returns a completed command if applicable
func (p *OSC133CommandParser) handleOSC133Marker(userID string, marker *OSC133Marker) *CommandEntry {
	p.mu.Lock()
	defer p.mu.Unlock()

	session := p.sessions[userID]
	if session == nil {
		return nil
	}

	// Mark that we've seen OSC 133 markers
	session.HasOSC133 = true

	p.logger.Debug("[OSC133] Marker received",
		"user_id", userID,
		"type", marker.Type,
		"data", marker.Data,
		"state", session.State,
	)

	switch marker.Type {
	case OSC133PromptStart:
		session.State = OSC133StateInPrompt

	case OSC133PreExec, OSC133PreExecAlt:
		session.State = OSC133StateExecuting
		session.CommandStart = marker.Timestamp
		// Use PendingCommand if available (set when Enter was pressed)
		if session.PendingCommand != "" {
			session.LastCommand = session.PendingCommand
		}
		p.logger.Info("[OSC133] Pre-exec marker, command ready",
			"user_id", userID,
			"command", session.LastCommand,
			"pending_command", session.PendingCommand,
		)
		session.PendingCommand = "" // Clear pending command

	case OSC133CommandExec:
		// Command is executing, but we don't have the command string yet
		// Wait for command exit marker

	case OSC133CommandExit:
		p.logger.Info("[OSC133] Exit marker received",
			"user_id", userID,
			"state", session.State,
			"exit_code_str", marker.Data,
			"last_command", session.LastCommand,
			"has_pending", session.PendingCommand != "",
		)
		// Accept exit marker in either Executing or Idle state (some shells send it late)
		if session.State == OSC133StateExecuting || session.State == OSC133StateIdle || session.State == OSC133StateInPrompt {
			session.CommandEnd = marker.Timestamp

			// Parse exit code from data
			exitCode := 0
			if marker.Data != "" {
				if _, err := fmt.Sscanf(marker.Data, "%d", &exitCode); err != nil {
					p.logger.Warn("[OSC133] Failed to parse exit code", "data", marker.Data, "error", err)
				}
			}
			session.ExitCode = exitCode

			// Calculate duration
			duration := session.CommandEnd.Sub(session.CommandStart)

			// Create command entry
			entry := &CommandEntry{
				Sequence:  len(session.CommandHistory) + 1,
				Command:   session.LastCommand,
				PWD:       session.CurrentDir,
				ExitCode:  exitCode,
				Duration:  duration,
				Timestamp: marker.Timestamp,
				StartTime: session.CommandStart,
				EndTime:   session.CommandEnd,
			}

			// Add to history, removing oldest entries in batches if limit exceeded
			if len(session.CommandHistory) >= MaxCommandHistory {
				session.CommandHistory = session.CommandHistory[CommandHistoryBatchSize:]
			}
			session.CommandHistory = append(session.CommandHistory, *entry)

			// Reset state
			session.CurrentCommand.Reset()
			session.State = OSC133StateIdle

			p.logger.Info("[OSC133] Command completed",
				"user_id", userID,
				"command", entry.Command,
				"exit_code", exitCode,
				"duration_ms", duration.Milliseconds(),
			)

			return entry
		}

	case OSC133PostExec:
		// Alternative post-exec marker
		session.State = OSC133StateIdle

	case OSC133EditorStart:
		session.InEditor = true
		session.EditorName = marker.Data
		// Clear any partial command buffers - editor keystrokes are not shell commands
		session.CurrentCommand.Reset()
		session.PendingCommand = ""
		p.logger.Info("[OSC133] Editor started",
			"user_id", userID,
			"editor", marker.Data,
		)

	case OSC133EditorEnd:
		session.InEditor = false
		session.EditorName = ""
		p.logger.Info("[OSC133] Editor exited",
			"user_id", userID,
		)
	}

	return nil
}

// GetCurrentCommand returns the command currently being typed
func (p *OSC133CommandParser) GetCurrentCommand(userID string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	session := p.sessions[userID]
	if session == nil {
		return ""
	}
	return session.CurrentCommand.String()
}

// GetLastCommand returns the last executed command
func (p *OSC133CommandParser) GetLastCommand(userID string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	session := p.sessions[userID]
	if session == nil {
		return ""
	}
	return session.LastCommand
}

// GetCommandHistory returns the command history for a session
func (p *OSC133CommandParser) GetCommandHistory(userID string, limit int) []CommandEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()

	session := p.sessions[userID]
	if session == nil {
		return nil
	}

	history := session.CommandHistory
	if limit > 0 && len(history) > limit {
		history = history[len(history)-limit:]
	}

	return history
}

// GetCurrentDir returns the current working directory for a session
func (p *OSC133CommandParser) GetCurrentDir(userID string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	session := p.sessions[userID]
	if session == nil {
		return ""
	}
	return session.CurrentDir
}

// HasOSC133Support returns whether OSC 133 markers have been detected for a session
func (p *OSC133CommandParser) HasOSC133Support(userID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	session := p.sessions[userID]
	if session == nil {
		return false
	}
	return session.HasOSC133
}

// UpdateCurrentDir updates the current working directory
func (p *OSC133CommandParser) UpdateCurrentDir(userID, dir string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	session := p.sessions[userID]
	if session != nil {
		session.CurrentDir = dir
	}
}

// SetCommandBuffer allows external sources to set the current command
// This is used when we detect the command from shell logging
func (p *OSC133CommandParser) SetCommandBuffer(userID, command string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	session := p.sessions[userID]
	if session != nil {
		session.LastCommand = command
		session.CurrentCommand.Reset()
		session.CurrentCommand.WriteString(command)
	}
}

// IsTyping returns whether the user is currently typing.
func (p *OSC133CommandParser) IsTyping(userID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	session := p.sessions[userID]
	if session == nil {
		return false
	}
	// User is typing if current command buffer has content
	return session.CurrentCommand.Len() > 0
}

// IsInEditor returns whether the user is currently in an editor
func (p *OSC133CommandParser) IsInEditor(userID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	session := p.sessions[userID]
	if session == nil {
		return false
	}
	return session.InEditor
}

// GetEditorName returns the name of the active editor (if any)
func (p *OSC133CommandParser) GetEditorName(userID string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	session := p.sessions[userID]
	if session == nil {
		return ""
	}
	return session.EditorName
}

// SetEditorMode sets the editor mode externally (fallback when OSC 133 G markers aren't available)
func (p *OSC133CommandParser) SetEditorMode(userID string, inEditor bool, editorName string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	session := p.sessions[userID]
	if session == nil {
		return
	}

	session.InEditor = inEditor
	session.EditorName = editorName
	if inEditor {
		// Clear any partial command buffers - editor keystrokes are not shell commands
		session.CurrentCommand.Reset()
		session.PendingCommand = ""
	}
	p.logger.Info("[OSC133] Editor mode set externally",
		"user_id", userID,
		"in_editor", inEditor,
		"editor_name", editorName,
	)
}

// ExtractOSC133Markers extracts all OSC 133 markers from a byte buffer
// Useful for testing and debugging
func (p *OSC133CommandParser) ExtractOSC133Markers(data []byte) []*OSC133Marker {
	var markers []*OSC133Marker
	pos := 0

	for pos < len(data) {
		// Look for ESC (0x1b)
		if data[pos] == 0x1b && pos+1 < len(data) && data[pos+1] == ']' {
			// Check for OSC 133
			if pos+6 < len(data) && bytes.HasPrefix(data[pos+2:], []byte("133;")) {
				// Find the ST (BEL: 0x07 or ESC: 0x1b 0x5c)
				stPos := pos + 6
				for stPos < len(data) && data[stPos] != 0x07 {
					stPos++
				}

				if stPos < len(data) {
					// Extract marker
					markerData := data[pos : stPos+1]
					if marker := p.extractOSC133Marker(markerData); marker != nil {
						markers = append(markers, marker)
					}
					pos = stPos + 1
					continue
				}
			}
		}
		pos++
	}

	return markers
}
