package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ── Whitelist ──

// whitelistedTools lists tools that are always auto-allowed.
var whitelistedTools = map[string]bool{
	"Read": true,
	"Glob": true,
	"Grep": true,
}

// whitelistedCommands lists command prefixes auto-allowed in Bash tool.
var whitelistedCommands = []string{
	"claudestra",
	"ls", "cat", "head", "tail", "find", "tree", "wc", "file", "stat",
	"grep", "rg", "ag", "fd",
	"echo", "printf", "pwd", "which", "env", "date",
}

// whitelistedGitSubcommands lists read-only git subcommands.
var whitelistedGitSubcommands = []string{
	"status", "log", "diff", "branch", "show", "tag", "remote",
	"stash list", "describe", "rev-parse", "ls-files", "ls-tree",
}

// IsWhitelisted checks if a tool call should be auto-allowed.
func IsWhitelisted(toolName string, toolInput map[string]interface{}) bool {
	// Allow if tool itself is whitelisted
	if whitelistedTools[toolName] {
		return true
	}

	// Check Bash command
	if toolName == "Bash" {
		command, _ := toolInput["command"].(string)
		return isWhitelistedCommand(command)
	}

	return false
}

func isWhitelistedCommand(command string) bool {
	command = strings.TrimSpace(command)
	if command == "" {
		return false
	}

	// Extract first token (before pipe/semicolon)
	firstCmd := extractFirstCommand(command)

	// Extract first word (executable) — also check basename for full path support
	firstWord := firstCmd
	if idx := strings.Index(firstCmd, " "); idx >= 0 {
		firstWord = firstCmd[:idx]
	}
	baseName := filepath.Base(firstWord)

	// Match command name or basename against whitelist
	for _, wl := range whitelistedCommands {
		if baseName == wl || firstCmd == wl || strings.HasPrefix(firstCmd, wl+" ") {
			return true
		}
	}

	// Check git read-only subcommands (supports full path git)
	if baseName == "git" || strings.HasPrefix(firstCmd, "git ") {
		// Extract args after "git " or "/usr/bin/git "
		gitArgs := ""
		if idx := strings.Index(firstCmd, "git "); idx >= 0 {
			gitArgs = firstCmd[idx+4:]
		}
		for _, sub := range whitelistedGitSubcommands {
			if gitArgs == sub || strings.HasPrefix(gitArgs, sub+" ") {
				return true
			}
		}
	}

	return false
}

// extractFirstCommand gets the first command from a pipeline or chain.
func extractFirstCommand(cmd string) string {
	// Split on pipe, semicolon, &&, etc. and take first command
	for _, sep := range []string{"|", "&&", "||", ";"} {
		if idx := strings.Index(cmd, sep); idx >= 0 {
			cmd = strings.TrimSpace(cmd[:idx])
			break
		}
	}
	return cmd
}

// ── Permission Request/Response Files ──

type PermissionRequest struct {
	ID        string `json:"id"`
	Tool      string `json:"tool"`
	Command   string `json:"command"`   // bash command or file path
	Agent     string `json:"agent"`     // requesting agent ID
	Timestamp string `json:"timestamp"`
}

type PermissionResponse struct {
	ID      string `json:"id"`
	Allowed bool   `json:"allowed"`
}

// PermissionsDir returns the permissions directory path for a workspace root.
func PermissionsDir(workspaceRoot string) string {
	return filepath.Join(workspaceRoot, ".orchestra", "permissions")
}

// WriteRequest writes a permission request file.
func WriteRequest(dir string, req *PermissionRequest) error {
	os.MkdirAll(dir, 0755)
	data, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "request-"+req.ID+".json"), data, 0644)
}

// ReadResponse reads a permission response file.
func ReadResponse(dir, id string) (*PermissionResponse, error) {
	data, err := os.ReadFile(filepath.Join(dir, "response-"+id+".json"))
	if err != nil {
		return nil, err
	}
	var resp PermissionResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// WriteResponse writes a permission response file.
func WriteResponse(dir string, resp *PermissionResponse) error {
	os.MkdirAll(dir, 0755)
	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "response-"+resp.ID+".json"), data, 0644)
}

// WaitForResponse polls for a response file with timeout.
func WaitForResponse(dir, id string, timeout time.Duration) (*PermissionResponse, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := ReadResponse(dir, id)
		if err == nil {
			return resp, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, fmt.Errorf("timeout waiting for permission response: %s", id)
}

// CleanupPermission removes request and response files for an ID.
func CleanupPermission(dir, id string) {
	os.Remove(filepath.Join(dir, "request-"+id+".json"))
	os.Remove(filepath.Join(dir, "response-"+id+".json"))
}
