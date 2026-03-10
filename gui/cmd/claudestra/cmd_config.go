package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"gui/internal"
)

// ── contract ──

func cmdContract() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "사용법: claudestra contract get|set <yaml>")
		os.Exit(1)
	}

	ws := mustWorkspace()
	sub := os.Args[2]

	switch sub {
	case "get":
		contract := ws.LoadContract()
		if contract == "" {
			fmt.Println("(계약서 없음)")
		} else {
			fmt.Println(contract)
		}

	case "set":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "사용법: claudestra contract set '<yaml>'")
			os.Exit(1)
		}
		yaml := os.Args[3]
		if err := ws.SaveContract(yaml); err != nil {
			fmt.Fprintf(os.Stderr, "저장 오류: %s\n", err)
			os.Exit(1)
		}
		fmt.Println("계약서 저장 완료")

	default:
		fmt.Fprintf(os.Stderr, "알 수 없는 contract 서브커맨드: %s\n", sub)
		os.Exit(1)
	}
}

// ── hook (PreToolUse 훅) ──

func cmdHook() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "사용법: claudestra hook pretooluse")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "pretooluse":
		cmdHookPreToolUse()
	default:
		fmt.Fprintf(os.Stderr, "알 수 없는 hook 서브커맨드: %s\n", os.Args[2])
		os.Exit(1)
	}
}

func cmdHookPreToolUse() {
	// Lead agent: no hook interference, use Claude's native permission system
	if os.Getenv("CLAUDESTRA_AGENT") == "" {
		os.Exit(0)
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(0)
	}

	var input struct {
		ToolName  string                 `json:"tool_name"`
		ToolInput map[string]interface{} `json:"tool_input"`
		CWD       string                 `json:"cwd"`
	}
	if err := json.Unmarshal(data, &input); err != nil {
		os.Exit(0)
	}

	if internal.IsWhitelisted(input.ToolName, input.ToolInput) {
		os.Exit(0)
	}

	root, err := internal.FindWorkspaceRoot()
	if err != nil {
		os.Exit(0)
	}

	permDir := internal.PermissionsDir(root)
	command := describeToolCall(input.ToolName, input.ToolInput)

	reqID := fmt.Sprintf("%d", time.Now().UnixNano())
	req := &internal.PermissionRequest{
		ID:        reqID,
		Tool:      input.ToolName,
		Command:   command,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if err := internal.WriteRequest(permDir, req); err != nil {
		os.Exit(0)
	}
	defer internal.CleanupPermission(permDir, reqID)

	resp, err := internal.WaitForResponse(permDir, reqID, 5*time.Minute)
	if err != nil {
		fmt.Fprintln(os.Stderr, "권한 승인 대기 시간 초과")
		os.Exit(2)
	}

	if resp.Allowed {
		os.Exit(0)
	} else {
		fmt.Fprintln(os.Stderr, "사용자가 거부했습니다")
		os.Exit(2)
	}
}

func describeToolCall(toolName string, toolInput map[string]interface{}) string {
	switch toolName {
	case "Bash":
		if cmd, ok := toolInput["command"].(string); ok {
			return cmd
		}
	case "Write":
		if fp, ok := toolInput["file_path"].(string); ok {
			return fmt.Sprintf("Write: %s", fp)
		}
	case "Edit":
		if fp, ok := toolInput["file_path"].(string); ok {
			return fmt.Sprintf("Edit: %s", fp)
		}
	}
	return fmt.Sprintf("%s: %v", toolName, toolInput)
}
