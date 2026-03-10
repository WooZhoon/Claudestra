package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gui/internal"
)

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

	var roles []string
	for _, p := range plans {
		roles = append(roles, p.Role)
	}
	ws.Init(roles)

	if err := ws.SaveRolePlans(plans); err != nil {
		fmt.Fprintf(os.Stderr, "저장 오류: %s\n", err)
		os.Exit(1)
	}

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
