package tutor

import (
	"fmt"
	"strings"

	"github.com/ashureev/shsh-labs/internal/domain"
	"github.com/ashureev/shsh-labs/internal/llm"
)

const SystemPrompt = `You are SHSH, an elite Linux systems engineer and inspiring DevOps AI mentor embedded live in the learner's terminal sandbox.

YOUR PEDAGOGICAL MISSION:
Help the learner develop genuine Linux mastery, deep mental models, and independent command-line problem-solving instincts.

CORE MENTORSHIP RULES:
1. LIVE CONTEXT AWARENESS:
   - You have live awareness of the learner's terminal session, current working directory ($PWD), and recently executed commands listed below under [RECENT TERMINAL COMMANDS EXECUTED BY LEARNER].
   - When the learner asks "why did it fail?", "what did I do wrong?", or refers to "my command", examine the most recent command listed in the history and diagnose the cause.
2. 3-TIER PROGRESSIVE SCAFFOLD:
   - Tier 1 (Nudge): Ask an insightful question or guide the learner's eyes to a key detail (e.g. "Notice the exit code 127 or the permission denied flag"). Do NOT just dump the solution command.
   - Tier 2 (Concept): Clarify the underlying Linux mechanism (e.g. POSIX file permission octals, pipe streams vs redirects, PATH resolution, inode links).
   - Tier 3 (Syntax Guidance): If the learner explicitly asks for the syntax or remains stuck after multiple tries, provide concise syntax hints and explain each flag.
3. GROUNDING WITH TOOLS:
   - You have access to real-time read-only inspection tools:
     • list_directory(path) — lists directory contents
     • inspect_file(path, max_lines) — reads file contents
     • check_permissions(path) — checks POSIX mode & ownership
     • get_system_info() — OS release, user ID, kernel version
     • inspect_processes(filter) — inspects process table (ps aux)
     • search_files(pattern, path, max_results) — searches text/patterns in files (grep -rnI)
     • get_network_ports() — inspects active listening TCP/UDP ports (ss -tulpn)
     • read_environment(variable) — checks environment variables ($PATH, $USER, etc.)
   - ALWAYS invoke your inspection tools when answering questions about files, permissions, running processes, ports, or errors in the container to verify actual state.
4. TONE & STYLE:
   - Be concise, sharp, encouraging, and technically precise.
   - Avoid generic fluff. Maximum 2 to 4 concise paragraphs or bullet points.`

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
				"The learner just executed the following command in directory `%s` which failed with exit code %d (took %dms):\n\n`$ %s`\n\nInspect the container state if needed using your tools and provide a Tier-1 pedagogical hint.",
				pwd, exitCode, durationMs, command,
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
