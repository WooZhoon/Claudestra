package main

import (
	"encoding/json"
	"fmt"
	"os"

	"gui/internal"
)

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

// ── lead-session ──

func cmdLeadSession() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "사용법: claudestra lead-session <request>")
		os.Exit(1)
	}

	request := os.Args[2]
	ws := mustWorkspace()
	plans := mustPlans(ws)

	lead := internal.NewLeadAgent(ws.Root)

	self, _ := os.Executable()
	lead.CLIPath = self

	agents := ws.BuildAgentsFromPlans(plans, internal.BuildOptions{LoadContract: true})
	for _, agent := range agents {
		lead.AddAgent(agent)
	}

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
