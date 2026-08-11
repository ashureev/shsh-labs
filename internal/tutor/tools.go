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

	default:
		return fmt.Sprintf("Unknown tool %q", name), nil
	}
}
