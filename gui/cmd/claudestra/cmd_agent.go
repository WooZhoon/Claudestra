package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"gui/internal"
)

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
	agents := ws.BuildAgentsFromPlans(plans, internal.BuildOptions{LoadContract: true})

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
	if len(os.Args) >= 4 && os.Args[2] == "--run-job" {
		cmdRunJob(os.Args[3])
		return
	}

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
	agents := ws.BuildAgentsFromPlans(plans, internal.BuildOptions{LoadContract: true})

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

func cmdAssignAsync(ws *internal.Workspace, agentID, instruction string) {
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

	self, _ := os.Executable()
	cmd := exec.Command(self, "assign", "--run-job", jobID)
	cmd.Dir = ws.Root
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "백그라운드 실행 실패: %s\n", err)
		os.Exit(1)
	}

	job.PID = cmd.Process.Pid
	internal.SaveJob(ws.JobsDir, job)

	fmt.Println(jobID)

	cmd.Process.Release()
}

func cmdRunJob(jobID string) {
	ws := mustWorkspace()

	job, err := internal.LoadJob(ws.JobsDir, jobID)
	if err != nil {
		os.Exit(1)
	}

	plans := mustPlans(ws)
	agents := ws.BuildAgentsFromPlans(plans, internal.BuildOptions{LoadContract: true})

	agent, ok := agents[job.Agent]
	if !ok {
		internal.FinishJob(ws.JobsDir, job, "error", "알 수 없는 에이전트: "+job.Agent)
		os.Exit(1)
	}

	agent.Reset()
	result := agent.Run(job.Instruction)

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

	if job.PID > 0 {
		syscall.Kill(-job.PID, syscall.SIGTERM)
	}

	internal.FinishJob(ws.JobsDir, job, "cancelled", "사용자에 의해 중지됨")
	fmt.Printf("작업 중지 완료: %s (%s)\n", job.ID, job.Agent)
}
