package tutor

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ashureev/shsh-labs/internal/llm"
)

// ContainerRunner executes non-interactive commands in a sandbox container.
type ContainerRunner interface {
	ExecCommand(ctx context.Context, containerID string, cmd []string) (stdout string, exitCode int, err error)
}

// ToolRegistry manages read-only inspection tools for the AI mentor.
type ToolRegistry struct {
	runner ContainerRunner
}

// NewToolRegistry creates a new ToolRegistry instance.
func NewToolRegistry(runner ContainerRunner) *ToolRegistry {
	return &ToolRegistry{runner: runner}
}

// GetDefinitions returns the LLM tool schemas.
func (r *ToolRegistry) GetDefinitions() []llm.ToolDefinition {
	return []llm.ToolDefinition{
		{
			Name:        "list_directory",
			Description: "Lists files and subdirectories in a given directory path",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The absolute or relative directory path to list",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "inspect_file",
			Description: "Reads the first N lines of a file (read-only)",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file to read",
					},
					"max_lines": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of lines to read (default 50)",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "check_permissions",
			Description: "Checks file/directory ownership and POSIX permissions (ls -ld)",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to inspect permissions for",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "get_system_info",
			Description: "Returns current OS release, user info (whoami, id), and kernel version",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "inspect_processes",
			Description: "Inspects running container processes and background tasks (ps aux)",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"filter": map[string]interface{}{
						"type":        "string",
						"description": "Optional process name or keyword filter to search for in process table",
					},
				},
			},
		},
		{
			Name:        "search_files",
			Description: "Searches text or error patterns recursively inside files and logs (grep -rnI)",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pattern": map[string]interface{}{
						"type":        "string",
						"description": "Text pattern or regex to search for in files",
					},
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Directory or file path to search within (default '.')",
					},
					"max_results": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum matching lines to return (default 30)",
					},
				},
				"required": []string{"pattern"},
			},
		},
		{
			Name:        "get_network_ports",
			Description: "Inspects active listening network ports and server daemons (ss -tulpn)",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "read_environment",
			Description: "Inspects active environment variables ($PATH, $USER, etc.) with credential redaction",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"variable": map[string]interface{}{
						"type":        "string",
						"description": "Optional specific environment variable name (e.g. PATH, SHELL, HOME)",
					},
				},
			},
		},
	}
}

// Execute runs the requested tool inside the sandbox container.
func (r *ToolRegistry) Execute(ctx context.Context, containerID string, name string, rawArgs string) (string, error) {
	if r.runner == nil {
		return "", fmt.Errorf("container runner not configured")
	}

	var args map[string]interface{}
	if rawArgs != "" {
		if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
			return "", fmt.Errorf("invalid tool arguments json: %w", err)
		}
	} else {
		args = make(map[string]interface{})
	}

	switch name {
	case "list_directory":
		path := "/"
		if p, ok := args["path"].(string); ok && p != "" {
			path = p
		}
		out, _, err := r.runner.ExecCommand(ctx, containerID, []string{"ls", "-la", path})
		if err != nil {
			return fmt.Sprintf("Error listing directory: %v", err), nil
		}
		return strings.TrimSpace(out), nil

	case "inspect_file":
		path, _ := args["path"].(string)
		if path == "" {
			return "Error: path is required", nil
		}
		maxLines := 50
		if ml, ok := args["max_lines"].(float64); ok && ml > 0 {
			maxLines = int(ml)
		}
		out, _, err := r.runner.ExecCommand(ctx, containerID, []string{"head", "-n", strconv.Itoa(maxLines), path})
		if err != nil {
			return fmt.Sprintf("Error reading file: %v", err), nil
		}
		return strings.TrimSpace(out), nil

	case "check_permissions":
		path, _ := args["path"].(string)
		if path == "" {
			path = "."
		}
		out, _, err := r.runner.ExecCommand(ctx, containerID, []string{"ls", "-ld", path})
		if err != nil {
			return fmt.Sprintf("Error checking permissions: %v", err), nil
		}
		return strings.TrimSpace(out), nil

	case "get_system_info":
		out, _, err := r.runner.ExecCommand(ctx, containerID, []string{"sh", "-c", "whoami; id; uname -a; cat /etc/os-release | head -n 4"})
		if err != nil {
			return fmt.Sprintf("Error getting system info: %v", err), nil
		}
		return strings.TrimSpace(out), nil

	case "inspect_processes":
		filter, _ := args["filter"].(string)
		var cmd []string
		if filter != "" {
			cmd = []string{"sh", "-c", fmt.Sprintf("ps aux | grep -i %s | grep -v grep | head -n 25", strconv.Quote(filter))}
		} else {
			cmd = []string{"sh", "-c", "ps aux --sort=-%cpu | head -n 25"}
		}
		out, _, err := r.runner.ExecCommand(ctx, containerID, cmd)
		if err != nil {
			return fmt.Sprintf("Error inspecting processes: %v", err), nil
		}
		return strings.TrimSpace(out), nil

	case "search_files":
		pattern, _ := args["pattern"].(string)
		if pattern == "" {
			return "Error: pattern is required", nil
		}
		path := "."
		if p, ok := args["path"].(string); ok && p != "" {
			path = p
		}
		maxResults := 30
		if mr, ok := args["max_results"].(float64); ok && mr > 0 {
			maxResults = int(mr)
		}

		cmd := []string{"sh", "-c", fmt.Sprintf("grep -rnI --exclude-dir='.git' -m %d %s %s 2>/dev/null | head -n %d", maxResults, strconv.Quote(pattern), strconv.Quote(path), maxResults)}
		out, _, err := r.runner.ExecCommand(ctx, containerID, cmd)
		if err != nil {
			return fmt.Sprintf("Error searching files: %v", err), nil
		}
		if strings.TrimSpace(out) == "" {
			return fmt.Sprintf("No matches found for pattern %q in %s", pattern, path), nil
		}
		return strings.TrimSpace(out), nil

	case "get_network_ports":
		out, _, err := r.runner.ExecCommand(ctx, containerID, []string{"sh", "-c", "ss -tulpn 2>/dev/null || netstat -tlpn 2>/dev/null || cat /proc/net/tcp | head -n 20"})
		if err != nil {
			return fmt.Sprintf("Error inspecting network ports: %v", err), nil
		}
		return strings.TrimSpace(out), nil

	case "read_environment":
		variable, _ := args["variable"].(string)
		var cmd []string
		if variable != "" {
			cmd = []string{"printenv", variable}
		} else {
			cmd = []string{"sh", "-c", "env | grep -v -i 'KEY\\|SECRET\\|TOKEN\\|PASS\\|AUTH' | sort | head -n 35"}
		}
		out, _, err := r.runner.ExecCommand(ctx, containerID, cmd)
		if err != nil {
			return fmt.Sprintf("Error reading environment: %v", err), nil
		}
		return strings.TrimSpace(out), nil

	default:
		return fmt.Sprintf("Unknown tool %q", name), nil
	}
}
