package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"gui/internal"
)

// ── Status/Detail Types ──

type AgentStatusInfo struct {
	ID         string `json:"id"`
	Role       string `json:"role"`
	Status     string `json:"status"`
	IsConsumer bool   `json:"isConsumer"`
}

type AgentDetailInfo struct {
	ID           string   `json:"id"`
	Role         string   `json:"role"`
	Status       string   `json:"status"`
	IsConsumer   bool     `json:"isConsumer"`
	Instruction  string   `json:"instruction"`
	Output       string   `json:"output"`
	Logs         string   `json:"logs"` // execution logs read from JSONL
	AllowedTools []string `json:"allowedTools"`
}

// ── Detail Queries ──

func (a *App) GetAgentDetail(agentID string) *AgentDetailInfo {
	agent, ok := a.agents[agentID]
	if !ok {
		return nil
	}

	// Read instruction, output, and real-time logs from JSONL
	instruction := agent.LastInstruction
	output := agent.Output
	var logs string

	if a.workspace != nil {
		logPath := filepath.Join(a.workspace.LogsDir, agentID+".jsonl")
		if entries, err := readJSONLEntries(logPath); err == nil && len(entries) > 0 {
			// Merge consecutive chunks of the same type into log lines
			var logLines []string
			var lastType string
			for _, e := range entries {
				// Append to last line if same consecutive type
				if e.Type == lastType && (e.Type == "thinking" || e.Type == "text") && len(logLines) > 0 {
					logLines[len(logLines)-1] += e.Message
					continue
				}
				lastType = e.Type
				switch e.Type {
				case "thinking":
					logLines = append(logLines, "💭 "+e.Message)
				case "tool":
					logLines = append(logLines, "🔧 "+e.Message)
				case "text":
					logLines = append(logLines, e.Message)
				case "status":
					logLines = append(logLines, "📌 "+e.Message)
				}
			}
			logs = strings.Join(logLines, "\n")

			// If output is empty, concatenate all text entries
			if output == "" {
				var textParts []string
				for _, e := range entries {
					if e.Type == "text" {
						textParts = append(textParts, e.Message)
					}
				}
				if len(textParts) > 0 {
					output = strings.Join(textParts, "")
				}
			}
		}

		// Read instruction file (written by CLI process)
		if instruction == "" {
			instrFile := filepath.Join(agent.Config.WorkDir, ".agent-instruction")
			if data, err := os.ReadFile(instrFile); err == nil {
				instruction = strings.TrimSpace(string(data))
			}
		}
	}

	// Read latest status from file (may have been updated by CLI process)
	status := string(agent.Status)
	statusFile := filepath.Join(agent.Config.WorkDir, ".agent-status")
	if data, err := os.ReadFile(statusFile); err == nil {
		s := strings.TrimSpace(string(data))
		if s != "" {
			status = s
		}
	}

	return &AgentDetailInfo{
		ID:           agent.Config.AgentID,
		Role:         agent.Config.Role,
		Status:       status,
		IsConsumer:   agent.Config.IsConsumer,
		Instruction:  instruction,
		Output:       output,
		Logs:         logs,
		AllowedTools: agent.Config.AllowedTools,
	}
}

// readJSONLEntries reads log entries from a JSONL file.
func readJSONLEntries(path string) ([]internal.LogEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []internal.LogEntry
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e internal.LogEntry
		if err := json.Unmarshal([]byte(line), &e); err == nil {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

func (a *App) GetAgentStatuses() []AgentStatusInfo {
	var statuses []AgentStatusInfo
	for _, agent := range a.agents {
		// 디스크에서 최신 상태 읽기 (.agent-status 파일)
		status := string(agent.Status)
		statusFile := filepath.Join(agent.Config.WorkDir, ".agent-status")
		if data, err := os.ReadFile(statusFile); err == nil {
			if s := strings.TrimSpace(string(data)); s != "" {
				status = s
				agent.Status = internal.AgentStatus(s)
			}
		}
		statuses = append(statuses, AgentStatusInfo{
			ID:         agent.Config.AgentID,
			Role:       agent.Config.Role,
			Status:     status,
			IsConsumer: agent.Config.IsConsumer,
		})
	}
	return statuses
}

func (a *App) GetLocks() map[string]string {
	if a.workspace == nil {
		return nil
	}
	registry := internal.NewFileLockRegistry(a.workspace.LocksDir)
	return registry.ListLocks()
}
