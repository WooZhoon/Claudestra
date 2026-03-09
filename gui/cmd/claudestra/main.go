package main

import (
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
  claudestra hook pretooluse          PreToolUse 훅 (stdin에서 JSON 읽음)`)
}

// ── 헬퍼 ──

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
	// team (인자 없음) → 목록 출력
	// team set '<json>' → 팀 설정
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

	// 유효성 검증
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

	// 디렉토리 생성
	var roles []string
	for _, p := range plans {
		roles = append(roles, p.Role)
	}
	ws.Init(roles)

	// 팀 저장
	if err := ws.SaveRolePlans(plans); err != nil {
		fmt.Fprintf(os.Stderr, "저장 오류: %s\n", err)
		os.Exit(1)
	}

	// 이데아 저장
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
	// --run-job <job-id> : 내부용 (백그라운드 프로세스가 호출)
	if len(os.Args) >= 4 && os.Args[2] == "--run-job" {
		cmdRunJob(os.Args[3])
		return
	}

	// 인자 파싱: --async 플래그 감지
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

// 동기 실행: stdout에 스트리밍 + JSONL 듀얼 라이트
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

// 비동기 실행: job 생성 → 자기 자신을 --run-job으로 재실행 → job-id 즉시 반환
func cmdAssignAsync(ws *internal.Workspace, agentID, instruction string) {
	// 1. job 파일 생성
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

	// 2. 자기 자신을 detached subprocess로 재실행
	self, _ := os.Executable()
	cmd := exec.Command(self, "assign", "--run-job", jobID)
	cmd.Dir = ws.Root
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // 부모로부터 detach

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "백그라운드 실행 실패: %s\n", err)
		os.Exit(1)
	}

	// 3. PID 기록
	job.PID = cmd.Process.Pid
	internal.SaveJob(ws.JobsDir, job)

	// 4. 부모는 job-id 출력 후 즉시 종료
	fmt.Println(jobID)

	// detached 프로세스이므로 Wait 불필요 — 즉시 반환
	cmd.Process.Release()
}

// --run-job: 백그라운드에서 실제 에이전트 실행
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

	// 실행
	agent.Reset()
	result := agent.Run(job.Instruction)

	// job 완료 처리
	status := "done"
	if agent.Status == internal.StatusError {
		status = "error"
	}
	internal.FinishJob(ws.JobsDir, job, status, result)
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

	// LeadAgent 구성
	lead := internal.NewLeadAgent(ws.Root)

	// CLI 바이너리 경로 설정
	self, _ := os.Executable()
	lead.CLIPath = self

	// 에이전트 구성
	agents := ws.BuildAgentsFromPlans(plans)
	for _, agent := range agents {
		lead.AddAgent(agent)
	}

	// 단일 세션 실행
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

// cmdHookPreToolUse: PreToolUse 훅 — stdin에서 JSON 읽고 화이트리스트 체크
func cmdHookPreToolUse() {
	// stdin에서 훅 입력 읽기
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

	// 화이트리스트 체크 → 허용
	if internal.IsWhitelisted(input.ToolName, input.ToolInput) {
		os.Exit(0)
	}

	// 화이트리스트에 없음 → GUI 승인 요청
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

	// 응답 대기 (최대 5분)
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
