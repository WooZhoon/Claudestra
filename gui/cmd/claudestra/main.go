package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"gui/internal"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "status":
		cmdStatus()
	case "team":
		cmdTeam()
	case "session":
		cmdSession()
	case "issues":
		cmdIssues()
	case "contract":
		cmdContract()
	case "idea":
		cmdIdea()
	case "output":
		cmdOutput()
	case "assign":
		cmdAssign()
	case "lead-session":
		cmdLeadSession()
	case "wait":
		cmdWait()
	case "stop":
		cmdStop()
	case "hook":
		cmdHook()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "알 수 없는 명령어: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`claudestra — Claudestra CLI

사용법:
  claudestra status                    팀원 전체 상태 출력
  claudestra team                      팀원 목록 출력
  claudestra team set <json>           팀 구성 설정
  claudestra session get               세션 메모리 읽기
  claudestra session update <json>     세션 메모리 갱신
  claudestra issues                    미해결 이슈 목록
  claudestra contract get              계약서 조회
  claudestra contract set <yaml>       계약서 설정
  claudestra idea <agent>              에이전트 이데아 출력
  claudestra output <agent>            에이전트 최근 출력
  claudestra assign <agent> <instr>    팀원에게 태스크 실행 (동기)
  claudestra lead-session <request>   단일 세션 모드 (팀장이 CLI로 전체 관리)
  claudestra wait <agent...>          팀원 완료 대기 (RUNNING→DONE/ERROR)
  claudestra stop <agent|job-id>     실행 중인 작업 중지
  claudestra hook pretooluse          PreToolUse 훅 (stdin에서 JSON 읽음)`)
}

// ── Helpers ──

func mustWorkspace() *internal.Workspace {
	root, err := internal.FindWorkspaceRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "오류: %s\n", err)
		os.Exit(1)
	}
	return internal.NewWorkspace(root)
}

func mustPlans(ws *internal.Workspace) []internal.RolePlan {
	plans := ws.LoadRolePlans()
	if len(plans) == 0 {
		fmt.Fprintln(os.Stderr, "오류: 팀이 구성되지 않았습니다. GUI에서 먼저 프로젝트를 초기화하세요.")
		os.Exit(1)
	}
	return plans
}

func printJSON(v interface{}) {
	data, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(data))
}

// ── status ──

func cmdStatus() {
	ws := mustWorkspace()
	plans := mustPlans(ws)

	for _, plan := range plans {
		statusFile := ws.Root + "/" + plan.Directory + "/.agent-status"
		status := "IDLE"
		if data, err := os.ReadFile(statusFile); err == nil {
			status = strings.TrimSpace(string(data))
		}
		icon := "⚪"
		switch status {
		case "RUNNING":
			icon = "🔵"
		case "DONE":
			icon = "✅"
		case "ERROR":
			icon = "❌"
		}
		fmt.Printf("%-30s %s %s\n", plan.Role, status, icon)
	}
}

// ── team ──

func cmdTeam() {
	// No args → list team
	// "set" subcommand → configure team
	if len(os.Args) >= 3 && os.Args[2] == "set" {
		cmdTeamSet()
		return
	}

	ws := mustWorkspace()
	plans := ws.LoadRolePlans()
	if len(plans) == 0 {
		fmt.Println("[]")
		return
	}

	type teamEntry struct {
		Role        string `json:"role"`
		Type        string `json:"type"`
		Directory   string `json:"directory"`
		Description string `json:"description"`
	}
	var entries []teamEntry
	for _, p := range plans {
		entries = append(entries, teamEntry{
			Role:        p.Role,
			Type:        p.Type,
			Directory:   p.Directory,
			Description: p.Description,
		})
	}
	printJSON(entries)
}

func cmdTeamSet() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "사용법: claudestra team set '<json>'")
		fmt.Fprintln(os.Stderr, `예시: claudestra team set '[{"role":"developer","description":"개발자","type":"producer","directory":"developer"}]'`)
		os.Exit(1)
	}

	jsonStr := os.Args[3]
	var plans []internal.RolePlan
	if err := json.Unmarshal([]byte(jsonStr), &plans); err != nil {
		fmt.Fprintf(os.Stderr, "JSON 파싱 오류: %s\n", err)
		os.Exit(1)
	}

	// Validate
	for i, p := range plans {
		if p.Role == "" || p.Directory == "" {
			fmt.Fprintf(os.Stderr, "오류: plans[%d]에 role 또는 directory가 비어있습니다\n", i)
			os.Exit(1)
		}
		if p.Type != "producer" && p.Type != "consumer" {
			plans[i].Type = "producer"
		}
	}

	ws := mustWorkspace()

	// Create directories
	var roles []string
	for _, p := range plans {
		roles = append(roles, p.Role)
	}
	ws.Init(roles)

	// Save team
	if err := ws.SaveRolePlans(plans); err != nil {
		fmt.Fprintf(os.Stderr, "저장 오류: %s\n", err)
		os.Exit(1)
	}

	// Save ideas
	for _, p := range plans {
		if p.Description != "" {
			ws.SaveIdea(p.Role, p.Description)
		}
	}

	fmt.Printf("팀 구성 완료: %d명\n", len(plans))
	for _, p := range plans {
		fmt.Printf("  - %s (%s) → ./%s/\n", p.Role, p.Type, p.Directory)
	}
}

// ── session ──

func cmdSession() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "사용법: claudestra session get|update <json>")
		os.Exit(1)
	}

	ws := mustWorkspace()
	sub := os.Args[2]

	switch sub {
	case "get":
		session := ws.LoadSession()
		printJSON(session)

	case "update":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "사용법: claudestra session update '<json>'")
			os.Exit(1)
		}
		jsonStr := os.Args[3]
		var session internal.Session
		if err := json.Unmarshal([]byte(jsonStr), &session); err != nil {
			fmt.Fprintf(os.Stderr, "JSON 파싱 오류: %s\n", err)
			os.Exit(1)
		}
		if err := ws.SaveSession(&session); err != nil {
			fmt.Fprintf(os.Stderr, "저장 오류: %s\n", err)
			os.Exit(1)
		}
		fmt.Println("세션 업데이트 완료")

	default:
		fmt.Fprintf(os.Stderr, "알 수 없는 session 서브커맨드: %s\n", sub)
		os.Exit(1)
	}
}

// ── issues ──

func cmdIssues() {
	ws := mustWorkspace()
	session := ws.LoadSession()

	var open []internal.OpenIssue
	for _, issue := range session.OpenIssues {
		if issue.Status == "open" {
			open = append(open, issue)
		}
	}
	if len(open) == 0 {
		fmt.Println("[]")
		return
	}
	printJSON(open)
}

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

// ── idea ──

func cmdIdea() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "사용법: claudestra idea <agent>")
		os.Exit(1)
	}

	ws := mustWorkspace()
	agentID := os.Args[2]
	idea := ws.LoadIdea(agentID)
	fmt.Println(idea)
}

// ── output ──

func cmdOutput() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "사용법: claudestra output <agent>")
		os.Exit(1)
	}

	ws := mustWorkspace()
	agentID := os.Args[2]
	plans := mustPlans(ws)
	agents := ws.BuildAgentsFromPlans(plans)

	agent, ok := agents[agentID]
	if !ok {
		fmt.Fprintf(os.Stderr, "알 수 없는 에이전트: %s\n", agentID)
		os.Exit(1)
	}

	if agent.Output == "" {
		fmt.Println("(출력 없음)")
	} else {
		fmt.Println(agent.Output)
	}
}

// ── assign ──

func cmdAssign() {
	// --run-job <job-id> : internal use (called by background process)
	if len(os.Args) >= 4 && os.Args[2] == "--run-job" {
		cmdRunJob(os.Args[3])
		return
	}

	// Parse args: detect --async flag
	args := os.Args[2:]
	async := false
	var filtered []string
	for _, a := range args {
		if a == "--async" {
			async = true
		} else {
			filtered = append(filtered, a)
		}
	}

	if len(filtered) < 2 {
		fmt.Fprintln(os.Stderr, "사용법: claudestra assign [--async] <agent> <instruction>")
		os.Exit(1)
	}

	agentID := filtered[0]
	instruction := filtered[1]

	ws := mustWorkspace()
	plans := mustPlans(ws)
	agents := ws.BuildAgentsFromPlans(plans)

	if _, ok := agents[agentID]; !ok {
		fmt.Fprintf(os.Stderr, "알 수 없는 에이전트: %s\n", agentID)
		fmt.Fprintln(os.Stderr, "사용 가능한 에이전트:")
		for id := range agents {
			fmt.Fprintf(os.Stderr, "  - %s\n", id)
		}
		os.Exit(1)
	}

	if async {
		cmdAssignAsync(ws, agentID, instruction)
	} else {
		cmdAssignSync(ws, agents, agentID, instruction)
	}
}

// cmdAssignSync runs an agent synchronously with stdout streaming + JSONL dual-write.
func cmdAssignSync(ws *internal.Workspace, agents map[string]*internal.Agent, agentID, instruction string) {
	agent := agents[agentID]
	agent.Reset()
	result := agent.Run(instruction, func(msg string) {
		if len(msg) > 0 && msg[0] == '\x01' {
			fmt.Print(msg[1:])
		} else {
			fmt.Println(msg)
		}
	})

	fmt.Println("\n--- RESULT ---")
	fmt.Println(result)
}

// cmdAssignAsync creates a job, re-executes itself with --run-job, and returns the job-id immediately.
func cmdAssignAsync(ws *internal.Workspace, agentID, instruction string) {
	// 1. Create job file
	jobID := internal.NewJobID()
	job := &internal.Job{
		ID:          jobID,
		Agent:       agentID,
		Status:      "running",
		Instruction: instruction,
		StartedAt:   "",
	}
	if err := internal.SaveJob(ws.JobsDir, job); err != nil {
		fmt.Fprintf(os.Stderr, "job 생성 실패: %s\n", err)
		os.Exit(1)
	}

	// 2. Re-execute self as detached subprocess
	self, _ := os.Executable()
	cmd := exec.Command(self, "assign", "--run-job", jobID)
	cmd.Dir = ws.Root
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // detach from parent

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "백그라운드 실행 실패: %s\n", err)
		os.Exit(1)
	}

	// 3. Record PID
	job.PID = cmd.Process.Pid
	internal.SaveJob(ws.JobsDir, job)

	// 4. Print job-id and exit immediately
	fmt.Println(jobID)

	// Detached process — no need to Wait
	cmd.Process.Release()
}

// cmdRunJob runs the actual agent in background (invoked by --run-job).
func cmdRunJob(jobID string) {
	ws := mustWorkspace()

	job, err := internal.LoadJob(ws.JobsDir, jobID)
	if err != nil {
		os.Exit(1)
	}

	plans := mustPlans(ws)
	agents := ws.BuildAgentsFromPlans(plans)

	agent, ok := agents[job.Agent]
	if !ok {
		internal.FinishJob(ws.JobsDir, job, "error", "알 수 없는 에이전트: "+job.Agent)
		os.Exit(1)
	}

	// Execute
	agent.Reset()
	result := agent.Run(job.Instruction)

	// Finalize job
	status := "done"
	if agent.Status == internal.StatusError {
		status = "error"
	}
	internal.FinishJob(ws.JobsDir, job, status, result)
}

// ── wait ──

func cmdWait() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "사용법: claudestra wait <agent1> [agent2 ...]")
		os.Exit(1)
	}

	ws := mustWorkspace()
	targets := os.Args[2:]

	// Poll every 2 seconds until all targets are no longer RUNNING
	for {
		allDone := true
		for _, target := range targets {
			statusFile := ws.Root + "/" + target + "/.agent-status"
			status := "IDLE"
			if data, err := os.ReadFile(statusFile); err == nil {
				status = strings.TrimSpace(string(data))
			}
			if status == "RUNNING" {
				allDone = false
				break
			}
		}
		if allDone {
			break
		}
		time.Sleep(2 * time.Second)
	}

	// Print final status
	for _, target := range targets {
		statusFile := ws.Root + "/" + target + "/.agent-status"
		status := "IDLE"
		if data, err := os.ReadFile(statusFile); err == nil {
			status = strings.TrimSpace(string(data))
		}
		fmt.Printf("%s: %s\n", target, status)
	}
}

// ── stop ──

func cmdStop() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "사용법: claudestra stop <agent|job-id>")
		os.Exit(1)
	}

	ws := mustWorkspace()
	target := os.Args[2]

	var job *internal.Job

	// hex 12자이면 job-id로 직접 로드
	if len(target) == 12 {
		if _, err := hex.DecodeString(target); err == nil {
			j, err := internal.LoadJob(ws.JobsDir, target)
			if err != nil {
				fmt.Fprintf(os.Stderr, "job을 찾을 수 없습니다: %s\n", target)
				os.Exit(1)
			}
			job = j
		}
	}

	// job-id가 아니면 agent 이름으로 running job 검색
	if job == nil {
		for _, j := range internal.ListJobs(ws.JobsDir) {
			if j.Agent == target && j.Status == "running" {
				job = j
				break
			}
		}
	}

	if job == nil {
		fmt.Println("실행 중인 작업이 없습니다")
		return
	}

	if job.Status != "running" {
		fmt.Printf("작업이 이미 종료되었습니다: %s (상태: %s)\n", job.ID, job.Status)
		return
	}

	// 프로세스 그룹에 SIGTERM 전송
	if job.PID > 0 {
		syscall.Kill(-job.PID, syscall.SIGTERM)
	}

	internal.FinishJob(ws.JobsDir, job, "cancelled", "사용자에 의해 중지됨")
	fmt.Printf("작업 중지 완료: %s (%s)\n", job.ID, job.Agent)
}

// ── lead-session ──

func cmdLeadSession() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "사용법: claudestra lead-session <request>")
		os.Exit(1)
	}

	request := os.Args[2]
	ws := mustWorkspace()
	plans := mustPlans(ws)

	// Configure LeadAgent
	lead := internal.NewLeadAgent(ws.Root)

	// Set CLI binary path
	self, _ := os.Executable()
	lead.CLIPath = self

	// Configure agents
	agents := ws.BuildAgentsFromPlans(plans)
	for _, agent := range agents {
		lead.AddAgent(agent)
	}

	// Run single session
	result := lead.RunLeadSession(request, func(msg string) {
		if len(msg) > 0 && msg[0] == '\x01' {
			fmt.Print(msg[1:])
		} else {
			fmt.Println(msg)
		}
	})

	fmt.Println("\n--- RESULT ---")
	fmt.Println(result)
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

// cmdHookPreToolUse handles the PreToolUse hook — reads JSON from stdin and checks whitelist.
func cmdHookPreToolUse() {
	// Read hook input from stdin
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(0) // 읽기 실패 시 허용
	}

	var input struct {
		ToolName  string                 `json:"tool_name"`
		ToolInput map[string]interface{} `json:"tool_input"`
		CWD       string                 `json:"cwd"`
	}
	if err := json.Unmarshal(data, &input); err != nil {
		os.Exit(0) // 파싱 실패 시 허용
	}

	// Allow if whitelisted
	if internal.IsWhitelisted(input.ToolName, input.ToolInput) {
		os.Exit(0)
	}

	// Not whitelisted — request GUI approval
	root, err := internal.FindWorkspaceRoot()
	if err != nil {
		os.Exit(0) // 워크스페이스 없으면 허용
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
		os.Exit(0) // 파일 작성 실패 시 허용
	}
	defer internal.CleanupPermission(permDir, reqID)

	// Wait for response (max 5 minutes)
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
