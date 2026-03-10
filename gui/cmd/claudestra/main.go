package main

import (
	"encoding/json"
	"fmt"
	"os"

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
