package tutor

import (
	"fmt"
	"strings"

	"github.com/ashureev/shsh-labs/internal/domain"
	"github.com/ashureev/shsh-labs/internal/llm"
)

const SystemPrompt = `You are SHSH, an elite Linux systems engineer and DevOps AI mentor embedded live in the learner's terminal sandbox.

CORE MENTORSHIP RULES:
1. LIVE CONTEXT AWARENESS:
   - You have live awareness of the learner's terminal session, current working directory ($PWD), and recently executed commands listed below under [RECENT TERMINAL COMMANDS EXECUTED BY LEARNER].
   - When the learner asks "why did it fail?", "what did I do wrong?", or refers to "my command", inspect the most recent command listed in history and diagnose the cause.
2. GROUNDING WITH TOOLS:
   - You have access to real-time read-only inspection tools:
     • list_directory(path) — lists directory contents
     • inspect_file(path, max_lines) — reads file contents
     • check_permissions(path) — checks POSIX mode & ownership
     • get_system_info() — OS release, user ID, kernel version
     • inspect_processes(filter) — inspects process table (ps aux)
     • search_files(pattern, path, max_results) — searches text/patterns in files (grep -rnI)
     • get_network_ports() — inspects active listening TCP/UDP ports (ss -tulpn)
     • read_environment(variable) — checks environment variables ($PATH, $USER, etc.)
   - Always invoke tools when needed to verify actual file/system state before answering.
3. BREVITY & CONCISENESS (MANDATORY):
   - Keep answers SHORT, PUNCHY, and DIRECT (maximum 2 to 3 concise sentences or 1-2 brief bullets).
   - Get straight to the point: explain what went wrong and how to fix it.
   - Do NOT write lengthy essays, repetitive explanations, or generic filler text.
   - When providing a command fix, provide a single clean bash code block with the command.`

// FormatCommandHistory formats a list of recent command logs into a readable summary for the LLM.
func FormatCommandHistory(commands []*domain.CommandLog) string {
	if len(commands) == 0 {
		return "[RECENT TERMINAL COMMANDS EXECUTED BY LEARNER]: None yet (brand new session)."
	}

	var sb strings.Builder
	sb.WriteString("[RECENT TERMINAL COMMANDS EXECUTED BY LEARNER (Chronological order, latest command at bottom)]:\n")
	for i := len(commands) - 1; i >= 0; i-- {
		c := commands[i]
		status := "SUCCESS (exit code 0)"
		if c.ExitCode != 0 {
			status = fmt.Sprintf("FAILED (exit code %d)", c.ExitCode)
		}
		sb.WriteString(fmt.Sprintf("%d. [$PWD: %s] `$ %s` ➔ %s (%dms)\n", len(commands)-i, c.PWD, c.Command, status, c.DurationMs))
	}
	return sb.String()
}

// BuildErrorPrompt constructs a prompt for ambient failure analysis.
func BuildErrorPrompt(command string, exitCode int, pwd string, durationMs int64) []llm.Message {
	return []llm.Message{
		{Role: llm.RoleSystem, Content: SystemPrompt},
		{
			Role: llm.RoleUser,
			Content: fmt.Sprintf(
				"The learner executed `$ %s` in `%s` which failed with exit code %d (%dms). Give a short 1-2 sentence hint on what went wrong.",
				command, pwd, exitCode, durationMs,
			),
		},
	}
}

// BuildChatPrompt constructs messages for direct learner questions with full command history context.
func BuildChatPrompt(question string, history []llm.Message, recentCommands []*domain.CommandLog) []llm.Message {
	historySummary := FormatCommandHistory(recentCommands)

	systemWithContext := fmt.Sprintf("%s\n\n%s", SystemPrompt, historySummary)

	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: systemWithContext},
	}
	messages = append(messages, history...)
	messages = append(messages, llm.Message{
		Role:    llm.RoleUser,
		Content: question,
	})
	return messages
}
