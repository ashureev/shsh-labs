package terminal

import (
	"bytes"
	"context"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ashureev/shsh-labs/internal/agent"
)

// Common prompt patterns used across all monitor implementations.
var promptPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\$\s+(.+)$`),                 // $ command
	regexp.MustCompile(`#\s+(.+)$`),                  // # command (root)
	regexp.MustCompile(`>\s+(.+)$`),                  // > command
	regexp.MustCompile(`\]\$\s+(.+)$`),               // ]$ command
	regexp.MustCompile(`bash-[\d.]+\$\s+(.+)$`),      // bash-5.1$ command
	regexp.MustCompile(`\w+@[\w-]+:[^$]+\$\s+(.+)$`), // user@host:path$ command
}

// cdPattern detects directory changes in output.
var cdPattern = regexp.MustCompile(`cd\s+(\S+)`)

// editorCommandPattern detects editor commands (vim, nano, etc.).
var editorCommandPattern = regexp.MustCompile(`^(?:\s*)\b(vim?|nano|emacs|less|more|man)\b`)

// Pre-computed lowercase error indicators for detectExitCode.
var lowerErrorIndicators [][]byte

func init() {
	errorIndicators := []string{
		"command not found",
		"no such file or directory",
		"permission denied",
		"invalid argument",
		"operation not permitted",
		"syntax error",
		"cannot access",
		"not recognized",
	}
	lowerErrorIndicators = make([][]byte, len(errorIndicators))
	for i, indicator := range errorIndicators {
		lowerErrorIndicators[i] = []byte(strings.ToLower(indicator))
	}
}

// MonitorState represents the current state of terminal monitoring.
type MonitorState int

const (
	// MonitorStateIdle indicates no active command processing.
	MonitorStateIdle MonitorState = iota // Waiting for command
	// MonitorStateTyping indicates user input is currently being entered.
	MonitorStateTyping // User is typing
	// MonitorStateExecuting indicates a command is currently executing.
	MonitorStateExecuting // Command is executing
	// MonitorStateCollecting indicates output is being buffered for analysis.
	MonitorStateCollecting // Collecting output
)

// SessionState tracks terminal state for a user session.
// This consolidates fields from SessionState, MonitorSession, UnifiedMonitorSession, and RobustMonitorSession.
type SessionState struct {
	UserID           string
	SessionID        string
	SessionKey       string
	ContainerID      string
	VolumePath       string
	CurrentDir       string
	LastCommand      string
	CommandCount     int
	PendingCommand   string
	OutputBuffer     bytes.Buffer
	CommandStartTime time.Time
	IsCollecting     bool
	IsTyping         bool
	LastActivity     time.Time
	HasOSC133        bool
	State            MonitorState
	InEditorMode     bool
	EditorName       string

	mu sync.RWMutex
}

// analysisJob represents a job for async AI analysis.
type analysisJob struct {
	ctx       context.Context
	userID    string
	sessionID string
	entry     *CommandEntry
	session   *SessionState
}

// TerminalMonitor provides unified terminal monitoring with OSC 133 shell integration
// and fallback to regex-based detection for shells without OSC 133 support.
// This consolidates the functionality from:
//   - monitor.go (original)
//   - monitor_v2.go (V2 with robust parser)
//   - monitor_unified.go (OSC 133 support)
//   - monitor_robust.go (improved race condition handling)
//
//nolint:revive // Public name retained for compatibility.
type TerminalMonitor struct {
	agentService   *agent.Service
	parser         *OSC133CommandParser
	sidebarChan    chan *agent.Response
	logger         *slog.Logger
	mu             sync.RWMutex
	sessions       map[string]*SessionState
	maxBufferSize  int
	jobChan        chan analysisJob
	workerWg       sync.WaitGroup
	workerPoolSize int
}

// NewTerminalMonitor creates a new unified terminal monitor.
func NewTerminalMonitor(agentService *agent.Service, sidebarChan chan *agent.Response, logger *slog.Logger) *TerminalMonitor {
	if logger == nil {
		logger = slog.Default()
	}

	tm := &TerminalMonitor{
		agentService:   agentService,
		parser:         NewOSC133CommandParser(logger),
		sidebarChan:    sidebarChan,
		logger:         logger,
		sessions:       make(map[string]*SessionState),
		maxBufferSize:  64 * 1024, // 64KB default buffer
		jobChan:        make(chan analysisJob, 100),
		workerPoolSize: 10,
	}

	// Start worker pool for async AI analysis
	for i := 0; i < tm.workerPoolSize; i++ {
		tm.workerWg.Add(1)
		go tm.analysisWorker()
	}

	return tm
}

// analysisWorker processes AI analysis jobs asynchronously.
func (tm *TerminalMonitor) analysisWorker() {
	defer tm.workerWg.Done()

	for job := range tm.jobChan {
		tm.processAnalysisJob(job)
	}
}

// processAnalysisJob processes a single analysis job.
func (tm *TerminalMonitor) processAnalysisJob(job analysisJob) {
	// Get output from buffer if not from OSC 133
	var output string
	if job.session != nil {
		job.session.mu.RLock()
		if job.session.IsCollecting {
			output = job.session.OutputBuffer.String()
		}
		job.session.mu.RUnlock()
	}

	// Get volume path from session
	var volumePath string
	if job.session != nil {
		volumePath = job.session.VolumePath
	}

	// Build terminal input for Agent with enhanced timing
	input := agent.TerminalInput{
		Command:    job.entry.Command,
		PWD:        job.entry.PWD,
		VolumePath: volumePath,
		ExitCode:   job.entry.ExitCode,
		Output:     output,
		Timestamp:  job.entry.Timestamp.Unix(),
		UserID:     job.userID,
		SessionID:  job.sessionID,
		Duration:   job.entry.Duration,
		HasOSC133:  tm.parser.HasOSC133Support(job.userID + ":" + job.sessionID),
	}

	// Process through Micro-Agent
	for response, err := range tm.agentService.ProcessTerminalInput(job.ctx, input) {
		if err != nil {
			tm.logger.Error("[MONITOR] Micro-Agent stream error",
				"user_id", job.userID,
				"command", job.entry.Command,
				"error", err,
			)
			break
		}

		tm.logger.Info("[MONITOR] Micro-Agent response chunk",
			"user_id", job.userID,
			"command", job.entry.Command,
			"response_is_nil", response == nil,
			"response_silent", response != nil && response.Silent,
			"response_type", func() string {
				if response == nil {
					return "nil"
				}
				return response.Type
			}(),
		)

		// Send to sidebar if not silent
		if response != nil && !response.Silent {
			response.UserID = job.userID
			response.SessionID = job.sessionID
			tm.sendToSidebar(job.ctx, job.userID, response)
		}
	}
}

// Stop gracefully shuts down the worker pool.
func (tm *TerminalMonitor) Stop() {
	close(tm.jobChan)
	tm.workerWg.Wait()
}

func monitorSessionKey(userID, sessionID string) string {
	return userID + ":" + sessionID
}

// RegisterSession registers a new terminal session for monitoring.
func (tm *TerminalMonitor) RegisterSession(userID, sessionID, containerID, volumePath string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	sessionKey := monitorSessionKey(userID, sessionID)

	tm.sessions[sessionKey] = &SessionState{
		UserID:       userID,
		SessionID:    sessionID,
		SessionKey:   sessionKey,
		ContainerID:  containerID,
		VolumePath:   volumePath,
		LastActivity: time.Now(),
		State:        MonitorStateIdle,
	}

	// Also register with the OSC 133 parser
	tm.parser.RegisterSession(sessionKey, containerID)

	tm.logger.Info("[MONITOR] Session registered",
		"user_id", userID,
		"session_id", sessionID,
		"container_id", containerID,
		"volume_path", volumePath,
	)
}

// UnregisterSession removes a session from monitoring.
func (tm *TerminalMonitor) UnregisterSession(userID, sessionID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	sessionKey := monitorSessionKey(userID, sessionID)

	delete(tm.sessions, sessionKey)
	tm.parser.UnregisterSession(sessionKey)

	tm.logger.Info("[MONITOR] Session unregistered", "user_id", userID, "session_id", sessionID)
}

// ProcessInput processes user keyboard input (WebSocket -> Container).
func (tm *TerminalMonitor) ProcessInput(ctx context.Context, userID, sessionID string, data []byte) {
	sessionKey := monitorSessionKey(userID, sessionID)
	tm.mu.RLock()
	session, exists := tm.sessions[sessionKey]
	tm.mu.RUnlock()

	if !exists {
		tm.logger.Debug("[MONITOR] ProcessInput: session not found", "user_id", userID, "session_id", sessionID)
		return
	}

	session.mu.Lock()
	session.LastActivity = time.Now()
	if !session.IsTyping {
		session.IsTyping = true
		session.mu.Unlock()
		if tm.agentService != nil {
			tm.agentService.UpdateSessionTypingStatus(ctx, userID, sessionID, true)
		}
	} else {
		session.mu.Unlock()
	}

	// Skip input processing if in editor mode (privacy protection)
	if session.InEditorMode {
		return
	}

	tm.logger.Info("[MONITOR] Processing input",
		"user_id", userID,
		"data_len", len(data),
		"data", string(data),
	)

	// Process through OSC 133 parser (for fallback detection)
	command, executed := tm.parser.ProcessInput(sessionKey, data)

	if executed && command != "" {
		tm.logger.Info("[MONITOR] Command executed (fallback)",
			"user_id", userID,
			"command", command,
			"osc133_support", tm.parser.HasOSC133Support(sessionKey),
		)

		// Detect if this is an editor command (fallback for missing OSC 133 G markers)
		if matches := editorCommandPattern.FindStringSubmatch(command); len(matches) > 1 {
			tm.mu.Lock()
			session.InEditorMode = true
			session.EditorName = matches[1]
			tm.mu.Unlock()
			// Sync to OSC 133 parser as well
			tm.parser.SetEditorMode(sessionKey, true, matches[1])
			tm.logger.Info("[MONITOR] Editor detected by command",
				"user_id", userID,
				"editor", matches[1],
				"command", command,
			)
			// Sync to learner session
			if tm.agentService != nil {
				tm.agentService.UpdateSessionEditorMode(ctx, userID, sessionID, true, matches[1])
			}
		}

		// Start collecting output for fallback path.
		// Use session.mu (not tm.mu) — all per-session field writes must use session.mu
		// so that processAnalysisJob can safely read them under the same lock.
		session.mu.Lock()
		session.PendingCommand = command
		session.CommandStartTime = time.Now()
		session.IsCollecting = true
		session.State = MonitorStateCollecting
		session.OutputBuffer.Reset()
		session.mu.Unlock()
	}
}

// ProcessOutput processes terminal output (Container -> WebSocket).
func (tm *TerminalMonitor) ProcessOutput(ctx context.Context, userID, sessionID string, data []byte) {
	sessionKey := monitorSessionKey(userID, sessionID)
	tm.mu.Lock()
	session, exists := tm.sessions[sessionKey]
	tm.mu.Unlock()

	if !exists {
		tm.logger.Warn("[MONITOR] ProcessOutput: session not found", "user_id", userID, "session_id", sessionID)
		return
	}

	session.mu.Lock()
	session.LastActivity = time.Now()
	session.mu.Unlock()

	previewLen := len(data)
	if previewLen > 100 {
		previewLen = 100
	}
	tm.logger.Info("[MONITOR] Processing output",
		"user_id", userID,
		"data_len", len(data),
		"data_preview", string(data[:previewLen]),
		"is_collecting", session.IsCollecting,
		"has_osc133", tm.parser.HasOSC133Support(sessionKey),
	)

	// Check for OSC 133 command completion first
	commandEntry := tm.parser.ProcessOutput(sessionKey, data)
	if commandEntry != nil {
		tm.logger.Info("[MONITOR] OSC 133 command entry detected",
			"user_id", userID,
			"command", commandEntry.Command,
			"exit_code", commandEntry.ExitCode,
		)
	} else {
		tm.logger.Debug("[MONITOR] No OSC 133 command entry detected", "user_id", userID)
	}

	// Check for editor mode transitions
	tm.mu.Lock()
	if tm.parser.IsInEditor(sessionKey) && !session.InEditorMode {
		session.InEditorMode = true
		session.EditorName = tm.parser.GetEditorName(sessionKey)
		tm.logger.Info("[MONITOR] Entered editor mode", "user_id", userID, "editor", session.EditorName)
		// Sync to learner session for silence rule
		tm.mu.Unlock()
		if tm.agentService != nil {
			tm.agentService.UpdateSessionEditorMode(ctx, userID, sessionID, true, session.EditorName)
		}
		tm.mu.Lock()
	} else if !tm.parser.IsInEditor(sessionKey) && session.InEditorMode {
		session.InEditorMode = false
		session.EditorName = ""
		tm.logger.Info("[MONITOR] Exited editor mode", "user_id", userID)
		// Sync to learner session for silence rule
		tm.mu.Unlock()
		if tm.agentService != nil {
			tm.agentService.UpdateSessionEditorMode(ctx, userID, sessionID, false, "")
		}
		tm.mu.Lock()
	}
	tm.mu.Unlock()

	// Check for editor command completion (editor exited)
	if session.InEditorMode && commandEntry != nil {
		tm.logger.Info("[MONITOR] Editor command completed, exiting editor mode",
			"user_id", userID,
			"command", commandEntry.Command,
			"exit_code", commandEntry.ExitCode,
		)
		session.InEditorMode = false
		session.EditorName = ""
		// Sync to OSC 133 parser
		tm.parser.SetEditorMode(sessionKey, false, "")
		// Sync to learner session
		if tm.agentService != nil {
			tm.agentService.UpdateSessionEditorMode(ctx, userID, sessionID, false, "")
		}
	}

	// Skip ALL processing if in editor mode (but we still need to detect exit above)
	if session.InEditorMode {
		return
	}

	if commandEntry != nil {
		// OSC 133 marker detected, command completed
		tm.logger.Info("[MONITOR] OSC 133 command completed",
			"user_id", userID,
			"command", commandEntry.Command,
			"exit_code", commandEntry.ExitCode,
			"duration_ms", commandEntry.Duration.Milliseconds(),
		)

		// Process the completed command
		tm.handleCommandExecuted(ctx, userID, sessionID, commandEntry)

		// Reset collection state under session.mu (not tm.mu) to match the read
		// path in processAnalysisJob which holds session.mu.
		session.mu.Lock()
		session.IsCollecting = false
		session.PendingCommand = ""
		session.State = MonitorStateIdle
		session.mu.Unlock()

		return
	}

	// Fallback: collect output if we're waiting for command completion.
	// session.mu guards OutputBuffer — same lock used by processAnalysisJob reader.
	if session.IsCollecting && session.PendingCommand != "" && !tm.parser.HasOSC133Support(sessionKey) {
		session.mu.Lock()
		session.OutputBuffer.Write(data)
		session.mu.Unlock()

		// Check for prompt pattern or timeout
		tm.checkFallbackCompletion(ctx, userID, sessionID, session)
	}

	// Extract PWD from output
	tm.extractPWDFromOutput(userID, sessionID, data)
}

// handleCommandExecuted processes a completed command with its metadata.
func (tm *TerminalMonitor) handleCommandExecuted(ctx context.Context, userID, sessionID string, entry *CommandEntry) {
	sessionKey := monitorSessionKey(userID, sessionID)
	// Skip editor commands - don't send them to AI
	if matches := editorCommandPattern.FindStringSubmatch(entry.Command); len(matches) > 1 {
		tm.logger.Info("[MONITOR] Skipping editor command for AI processing",
			"user_id", userID,
			"command", entry.Command,
			"editor", matches[1],
		)
		return
	}

	// Capture state under tm.mu, then release before making the gRPC call.
	// Holding tm.mu during a remote call would block all I/O processing if the
	// Python agent is slow or unreachable.
	tm.mu.Lock()
	session := tm.sessions[sessionKey]
	var wasTyping bool
	if session != nil {
		session.LastCommand = entry.Command
		session.CommandCount++
		if session.IsTyping {
			session.IsTyping = false
			wasTyping = true
		}
	}
	tm.mu.Unlock()

	// Call gRPC outside the lock.
	if wasTyping && tm.agentService != nil {
		tm.agentService.UpdateSessionTypingStatus(ctx, userID, sessionID, false)
	}

	// Truncate output for logging
	var outputPreview string
	if session != nil && session.IsCollecting {
		session.mu.RLock()
		outputPreview = session.OutputBuffer.String()
		session.mu.RUnlock()
		if len(outputPreview) > 200 {
			outputPreview = outputPreview[:200] + "..."
		}
	}

	tm.logger.Info("[MONITOR] Processing command",
		"user_id", userID,
		"command", entry.Command,
		"pwd", entry.PWD,
		"exit_code", entry.ExitCode,
		"duration_ms", entry.Duration.Milliseconds(),
		"output_len", func() int {
			if session != nil {
				session.mu.RLock()
				defer session.mu.RUnlock()
				return session.OutputBuffer.Len()
			}
			return 0
		}(),
		"output_preview", outputPreview,
	)

	// Enqueue job for async processing instead of blocking
	job := analysisJob{
		ctx:       ctx,
		userID:    userID,
		sessionID: sessionID,
		entry:     entry,
		session:   session,
	}

	select {
	case tm.jobChan <- job:
		tm.logger.Info("[MONITOR] Analysis job enqueued",
			"user_id", userID,
			"command", entry.Command,
		)
	default:
		tm.logger.Warn("[MONITOR] Analysis job queue full, dropping job",
			"user_id", userID,
			"command", entry.Command,
		)
	}
}

// checkFallbackCompletion checks if command completed using fallback detection.
func (tm *TerminalMonitor) checkFallbackCompletion(ctx context.Context, userID, sessionID string, session *SessionState) {
	duration := time.Since(session.CommandStartTime)
	outputSize := session.OutputBuffer.Len()

	// Timeout: 500ms with output or 2s without output
	timeoutReached := (duration > 500*time.Millisecond && outputSize > 0) ||
		(duration > 2*time.Second)

	if !timeoutReached {
		return
	}

	// Check for prompt pattern using bytes directly to avoid string allocation
	session.mu.RLock()
	outputBytes := session.OutputBuffer.Bytes()
	session.mu.RUnlock()
	promptDetected := tm.detectPromptBytes(outputBytes)

	if !promptDetected && !timeoutReached {
		return
	}

	// Create command entry
	sessionKey := monitorSessionKey(userID, sessionID)
	pwd := tm.parser.GetCurrentDir(sessionKey)
	entry := &CommandEntry{
		Sequence:  session.CommandCount + 1,
		Command:   session.PendingCommand,
		PWD:       pwd,
		ExitCode:  tm.detectExitCodeBytes(outputBytes),
		Duration:  duration,
		Timestamp: session.CommandStartTime,
		StartTime: session.CommandStartTime,
		EndTime:   time.Now(),
	}

	tm.logger.Info("[MONITOR] Fallback command completed",
		"user_id", userID,
		"command", entry.Command,
		"prompt_detected", promptDetected,
		"timeout_reached", timeoutReached,
		"duration_ms", duration.Milliseconds(),
	)

	// Process the command
	tm.handleCommandExecuted(ctx, userID, sessionID, entry)

	// Reset state
	session.IsCollecting = false
	session.PendingCommand = ""
	session.OutputBuffer.Reset()
	session.State = MonitorStateIdle
}

// sendToSidebar sends a response to the sidebar channel.
func (tm *TerminalMonitor) sendToSidebar(ctx context.Context, userID string, response *agent.Response) {
	tm.logger.Info("[MONITOR] Sending to sidebar",
		"user_id", userID,
		"type", response.Type,
		"content_len", len(response.Content),
		"channel_len", len(tm.sidebarChan),
	)

	select {
	case tm.sidebarChan <- response:
		tm.logger.Info("[MONITOR] Response sent to sidebar successfully",
			"user_id", userID,
			"type", response.Type,
		)
	case <-ctx.Done():
		tm.logger.Warn("[MONITOR] Context cancelled, response not sent",
			"user_id", userID,
		)
	default:
		tm.logger.Warn("[MONITOR] Sidebar channel full, response dropped",
			"user_id", userID,
			"channel_len", len(tm.sidebarChan),
		)
	}
}

// detectPrompt checks if output contains a shell prompt.
func (tm *TerminalMonitor) detectPrompt(output string) bool {
	for _, pattern := range promptPatterns {
		if pattern.MatchString(output) {
			return true
		}
	}
	return false
}

// detectPromptBytes checks if output contains a shell prompt (bytes version).
func (tm *TerminalMonitor) detectPromptBytes(output []byte) bool {
	for _, pattern := range promptPatterns {
		if pattern.Match(output) {
			return true
		}
	}
	return false
}

// detectExitCode attempts to determine exit code from output.
func (tm *TerminalMonitor) detectExitCode(output string) int {
	lowerOutput := strings.ToLower(output)
	for _, indicator := range lowerErrorIndicators {
		if strings.Contains(lowerOutput, string(indicator)) {
			return 1
		}
	}
	return 0
}

// detectExitCodeBytes attempts to determine exit code from output using bytes.
func (tm *TerminalMonitor) detectExitCodeBytes(output []byte) int {
	lowerOutput := bytes.ToLower(output)
	for _, indicator := range lowerErrorIndicators {
		if bytes.Contains(lowerOutput, indicator) {
			return 1
		}
	}
	return 0
}

// extractPWDFromOutput extracts current directory from output.
func (tm *TerminalMonitor) extractPWDFromOutput(userID, sessionID string, data []byte) {
	sessionKey := monitorSessionKey(userID, sessionID)
	output := string(data)

	// Look for cd commands
	if matches := cdPattern.FindStringSubmatch(output); len(matches) > 1 {
		tm.parser.UpdateCurrentDir(sessionKey, matches[1])
	}

	// Look for pwd output
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 0 && line[0] == '/' && !strings.Contains(line, " ") {
			tm.parser.UpdateCurrentDir(sessionKey, line)
			break
		}
	}
}

// GetCurrentCommand returns the command currently being typed.
func (tm *TerminalMonitor) GetCurrentCommand(userID, sessionID string) string {
	return tm.parser.GetCurrentCommand(monitorSessionKey(userID, sessionID))
}

// GetLastCommand returns the last executed command.
func (tm *TerminalMonitor) GetLastCommand(userID, sessionID string) string {
	return tm.parser.GetLastCommand(monitorSessionKey(userID, sessionID))
}

// GetCommandHistory returns the command history for a session.
func (tm *TerminalMonitor) GetCommandHistory(userID, sessionID string, limit int) []CommandEntry {
	return tm.parser.GetCommandHistory(monitorSessionKey(userID, sessionID), limit)
}

// HasOSC133Support returns whether OSC 133 markers have been detected.
func (tm *TerminalMonitor) HasOSC133Support(userID, sessionID string) bool {
	return tm.parser.HasOSC133Support(monitorSessionKey(userID, sessionID))
}

// IsInEditorMode returns whether the user is currently in editor mode.
func (tm *TerminalMonitor) IsInEditorMode(userID, sessionID string) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	session, exists := tm.sessions[monitorSessionKey(userID, sessionID)]
	if !exists {
		return false
	}
	return session.InEditorMode
}

// GetEditorName returns the name of the active editor (if any).
func (tm *TerminalMonitor) GetEditorName(userID, sessionID string) string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	session, exists := tm.sessions[monitorSessionKey(userID, sessionID)]
	if !exists {
		return ""
	}
	return session.EditorName
}

// UpdateTypingStatus updates whether the user is currently typing.
func (tm *TerminalMonitor) UpdateTypingStatus(userID, sessionID string, isTyping bool) {
	tm.mu.Lock()
	session, exists := tm.sessions[monitorSessionKey(userID, sessionID)]
	var changed bool
	if exists {
		changed = session.IsTyping != isTyping
		session.IsTyping = isTyping
	}
	tm.mu.Unlock()

	// Call gRPC outside the lock with a bounded timeout so a slow agent
	// cannot block callers indefinitely.
	if changed && exists && tm.agentService != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		tm.agentService.UpdateSessionTypingStatus(ctx, userID, sessionID, isTyping)
	}
}

// IsTyping returns whether the user is currently typing.
func (tm *TerminalMonitor) IsTyping(userID, sessionID string) bool {
	return tm.parser.IsTyping(monitorSessionKey(userID, sessionID))
}

// GetSessionState returns the current state for a session.
func (tm *TerminalMonitor) GetSessionState(userID, sessionID string) *SessionState {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.sessions[monitorSessionKey(userID, sessionID)]
}

// GetStats returns monitoring statistics for a session.
func (tm *TerminalMonitor) GetStats(userID, sessionID string) map[string]interface{} {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	sessionKey := monitorSessionKey(userID, sessionID)

	session, exists := tm.sessions[sessionKey]
	if !exists {
		return nil
	}

	return map[string]interface{}{
		"user_id":         session.UserID,
		"session_id":      session.SessionID,
		"command_count":   session.CommandCount,
		"last_command":    session.LastCommand,
		"is_collecting":   session.IsCollecting,
		"has_osc133":      tm.parser.HasOSC133Support(sessionKey),
		"current_dir":     tm.parser.GetCurrentDir(sessionKey),
		"last_activity":   session.LastActivity,
		"state":           session.State,
		"output_size":     session.OutputBuffer.Len(),
		"buffer_capacity": tm.maxBufferSize,
		"in_editor_mode":  session.InEditorMode,
		"editor_name":     session.EditorName,
	}
}
