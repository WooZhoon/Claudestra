package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gui/internal"

	"github.com/fsnotify/fsnotify"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// LogEvent is a structured log entry sent to the frontend.
type LogEvent struct {
	Type    string `json:"type"`    // "text", "thinking", "status"
	Message string `json:"message"`
}

type App struct {
	ctx       context.Context
	workspace *internal.Workspace
	lead      *internal.LeadAgent
	agents    map[string]*internal.Agent
	rolePlans []internal.RolePlan
}

func NewApp() *App {
	return &App{
		agents: make(map[string]*internal.Agent),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// ── Project Management ──

func (a *App) InitProject(projectDir string) error {
	a.workspace = internal.NewWorkspace(projectDir)
	a.lead = internal.NewLeadAgent(projectDir)
	a.agents = make(map[string]*internal.Agent)
	a.rolePlans = nil
	return nil
}

func (a *App) OpenProject(projectDir string) error {
	ws := internal.NewWorkspace(projectDir)

	config, err := ws.LoadConfig()
	if err != nil {
		return fmt.Errorf("프로젝트를 찾을 수 없습니다. '.orchestra' 폴더가 없습니다. 새 프로젝트로 시작하세요.")
	}

	a.workspace = ws
	a.lead = internal.NewLeadAgent(projectDir)
	a.agents = make(map[string]*internal.Agent)

	// Restore saved RolePlans
	a.rolePlans = ws.LoadRolePlans()
	if len(a.rolePlans) > 0 {
		a.buildTeamFromPlans(a.rolePlans)
	} else if len(config.Agents) > 0 {
		// Legacy compat: config.yaml with only roles
		a.buildLegacyTeam(config.Agents)
	}
	return nil
}

func (a *App) buildTeamFromPlans(plans []internal.RolePlan) {
	lockRegistry := internal.NewFileLockRegistry(a.workspace.LocksDir)
	a.lead = internal.NewLeadAgent(a.workspace.Root)
	a.agents = make(map[string]*internal.Agent)

	// Collect producer directories for consumer readRefs
	var producerDirs []string
	for _, p := range plans {
		if p.Type == "producer" {
			producerDirs = append(producerDirs, filepath.Join(a.workspace.Root, p.Directory))
		}
	}

	for _, plan := range plans {
		agentDir := filepath.Join(a.workspace.Root, plan.Directory)
		isConsumer := plan.Type == "consumer"

		var readRefs []string
		var allowedTools []string
		if isConsumer {
			readRefs = producerDirs
			allowedTools = internal.ConsumerTools
			// Consumers like doc_writer that need write access
			if strings.Contains(plan.Description, "문서") || strings.Contains(plan.Description, "작성") {
				allowedTools = append(internal.ConsumerTools, "Write")
			}
		} else {
			allowedTools = internal.ProducerTools
		}

		config := internal.AgentConfig{
			AgentID:      plan.Role,
			Role:         plan.Role,
			Idea:         plan.Description,
			WorkDir:      agentDir,
			ReadRefs:     readRefs,
			AllowedTools: allowedTools,
			IsConsumer:   isConsumer,
			LogPath:      filepath.Join(a.workspace.LogsDir, plan.Role+".jsonl"),
		}
		agent := internal.NewAgent(config, lockRegistry)
		a.agents[plan.Role] = agent
		a.lead.AddAgent(agent)
	}
}

// buildLegacyTeam handles legacy config format with hardcoded roles.
func (a *App) buildLegacyTeam(roles []string) {
	var plans []internal.RolePlan
	for _, role := range roles {
		idea := a.workspace.LoadIdea(role)
		roleType := "producer"
		if role == "reviewer" || role == "doc_writer" {
			roleType = "consumer"
		}
		plans = append(plans, internal.RolePlan{
			Role:        role,
			Description: idea,
			Type:        roleType,
			Directory:   role,
		})
	}
	a.rolePlans = plans
	a.buildTeamFromPlans(plans)
}

// ── Idea Management ──

func (a *App) GetIdea(role string) string {
	if a.workspace == nil {
		return ""
	}
	return a.workspace.LoadIdea(role)
}

func (a *App) UpdateIdea(role, idea string) error {
	if a.workspace == nil {
		return fmt.Errorf("프로젝트가 열려있지 않습니다")
	}
	return a.workspace.SaveIdea(role, idea)
}

// ── Contract ──

func (a *App) GetContract() string {
	if a.workspace == nil {
		return ""
	}
	return a.workspace.LoadContract()
}

// ── Status Query ──

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
		statuses = append(statuses, AgentStatusInfo{
			ID:         agent.Config.AgentID,
			Role:       agent.Config.Role,
			Status:     string(agent.Status),
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

// ── Single Session Mode (Phase D+E) ──

// RunLeadSession runs the entire workflow in a single Claude session via the lead agent.
// It watches .orchestra/logs/ with fsnotify to stream sub-agent JSONL output in real time.
func (a *App) RunLeadSession(userInput string) string {
	if a.lead == nil {
		return "프로젝트를 먼저 열어주세요."
	}

	logFn := func(msg string) {
		if len(msg) > 0 && msg[0] == '\x01' {
			runtime.EventsEmit(a.ctx, "log-append", msg[1:])
			return
		}
		evtType := "text"
		if strings.Contains(msg, "💭") || strings.Contains(msg, "🔧") || strings.Contains(msg, "📊") {
			evtType = "thinking"
		}
		runtime.EventsEmit(a.ctx, "log", LogEvent{Type: evtType, Message: msg})
	}

	// Ensure .orchestra directory exists (team setup is done by lead session via CLI)
	if a.workspace != nil {
		a.workspace.Init(nil)
	}

	// Auto-discover CLI path
	if a.lead.CLIPath == "" {
		a.lead.CLIPath = findCLI()
	}

	// Configure PreToolUse hook in project .claude/settings.json
	a.ensureHookSettings()

	// Start fsnotify log watcher.
	// Consecutive chunks of the same type are appended to avoid line breaks.
	var lastAgent, lastType string
	watcher := internal.NewLogWatcher(a.workspace.LogsDir, func(entry internal.LogEntry) {
		prefix := fmt.Sprintf("[%s] ", entry.Agent)
		isContinuation := entry.Agent == lastAgent && entry.Type == lastType &&
			(entry.Type == "thinking" || entry.Type == "text")

		switch entry.Type {
		case "thinking":
			if isContinuation {
				runtime.EventsEmit(a.ctx, "log-append", entry.Message)
			} else {
				runtime.EventsEmit(a.ctx, "log", LogEvent{Type: "thinking", Message: prefix + "💭 " + entry.Message})
			}
		case "tool":
			runtime.EventsEmit(a.ctx, "log", LogEvent{Type: "thinking", Message: prefix + "🔧 " + entry.Message})
		case "text":
			if isContinuation {
				runtime.EventsEmit(a.ctx, "log-append", entry.Message)
			} else {
				runtime.EventsEmit(a.ctx, "log", LogEvent{Type: "text", Message: prefix + entry.Message})
			}
		case "status":
			runtime.EventsEmit(a.ctx, "log", LogEvent{Type: "text", Message: prefix + "📌 " + entry.Message})
			runtime.EventsEmit(a.ctx, "team-updated", a.GetAgentStatuses())
		}
		lastAgent = entry.Agent
		lastType = entry.Type
	})
	if err := watcher.Start(); err != nil {
		logFn(fmt.Sprintf("[팀장] ⚠️ 로그 감시 시작 실패: %s", err))
	}
	defer watcher.Stop()

	// Watch permissions directory for GUI approval dialogs
	permDir := internal.PermissionsDir(a.workspace.Root)
	os.MkdirAll(permDir, 0755)
	permWatcher, permErr := fsnotify.NewWatcher()
	if permErr == nil {
		permWatcher.Add(permDir)
		permStop := make(chan struct{})
		go func() {
			for {
				select {
				case <-permStop:
					return
				case event, ok := <-permWatcher.Events:
					if !ok {
						return
					}
					if (event.Has(fsnotify.Write) || event.Has(fsnotify.Create)) &&
						strings.Contains(event.Name, "request-") &&
						strings.HasSuffix(event.Name, ".json") {
						// Read request file
						data, err := os.ReadFile(event.Name)
						if err != nil {
							continue
						}
						var req internal.PermissionRequest
						if err := json.Unmarshal(data, &req); err != nil {
							continue
						}
						runtime.EventsEmit(a.ctx, "permission-request", req)
					}
				case <-permWatcher.Errors:
				}
			}
		}()
		defer func() {
			close(permStop)
			permWatcher.Close()
		}()
	}

	// Watch team.json changes for immediate sidebar updates
	teamWatcher, err := fsnotify.NewWatcher()
	if err == nil {
		teamWatcher.Add(a.workspace.OrchestraDir)
		teamStop := make(chan struct{})
		go func() {
			for {
				select {
				case <-teamStop:
					return
				case event, ok := <-teamWatcher.Events:
					if !ok {
						return
					}
					if (event.Has(fsnotify.Write) || event.Has(fsnotify.Create)) &&
						strings.HasSuffix(event.Name, "team.json") {
						if plans := a.workspace.LoadRolePlans(); len(plans) > 0 {
							a.rolePlans = plans
							a.buildTeamFromPlans(plans)
							runtime.EventsEmit(a.ctx, "team-updated", a.GetAgentStatuses())
						}
					}
				case <-teamWatcher.Errors:
				}
			}
		}()
		defer func() {
			close(teamStop)
			teamWatcher.Close()
		}()
	}

	// Run single session
	result := a.lead.RunLeadSession(userInput, logFn)

	// Reload team after session (lead may have created team via team set)
	if plans := a.workspace.LoadRolePlans(); len(plans) > 0 && len(plans) != len(a.rolePlans) {
		a.rolePlans = plans
		a.buildTeamFromPlans(plans)
	}
	runtime.EventsEmit(a.ctx, "team-updated", a.GetAgentStatuses())

	return result
}

// ── Project Directory Selection ──

func (a *App) SelectDirectory() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "프로젝트 디렉토리 선택",
	})
	return dir, err
}

// ── Session Cancel ──

// CancelSession forcefully stops the running lead session.
func (a *App) CancelSession() {
	if a.lead != nil {
		a.lead.Cancel()
	}
}

// ── Permission Approval ──

// RespondPermission handles allow/disallow clicks from the frontend.
func (a *App) RespondPermission(id string, allowed bool) error {
	if a.workspace == nil {
		return fmt.Errorf("프로젝트가 열려있지 않습니다")
	}
	permDir := internal.PermissionsDir(a.workspace.Root)
	return internal.WriteResponse(permDir, &internal.PermissionResponse{
		ID:      id,
		Allowed: allowed,
	})
}

// ensureHookSettings writes PreToolUse hook config to project .claude/settings.json
func (a *App) ensureHookSettings() {
	if a.workspace == nil {
		return
	}
	cliPath := a.lead.CLIPath
	if cliPath == "" {
		cliPath = findCLI()
	}

	settingsDir := filepath.Join(a.workspace.Root, ".claude")
	settingsFile := filepath.Join(settingsDir, "settings.json")

	// Read existing settings
	var settings map[string]interface{}
	if data, err := os.ReadFile(settingsFile); err == nil {
		json.Unmarshal(data, &settings)
	}
	if settings == nil {
		settings = make(map[string]interface{})
	}

	// Build hook configuration
	hookCommand := cliPath + " hook pretooluse"
	expectedHook := map[string]interface{}{
		"type":    "command",
		"command": hookCommand,
		"timeout": 300,
	}

	hookEntry := map[string]interface{}{
		"matcher": ".*",
		"hooks":   []interface{}{expectedHook},
	}

	settings["hooks"] = map[string]interface{}{
		"PreToolUse": []interface{}{hookEntry},
	}

	os.MkdirAll(settingsDir, 0755)
	data, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(settingsFile, data, 0644)
}

// findCLI searches for the claudestra binary in common locations.
func findCLI() string {
	// 1. Search in PATH
	if path, err := exec.LookPath("claudestra"); err == nil {
		return path
	}
	// 2. ~/go/bin/ (go install location)
	if home, err := os.UserHomeDir(); err == nil {
		gobin := filepath.Join(home, "go", "bin", "claudestra")
		if _, err := os.Stat(gobin); err == nil {
			return gobin
		}
	}
	// 3. Next to the executable
	if self, err := os.Executable(); err == nil {
		beside := filepath.Join(filepath.Dir(self), "claudestra")
		if _, err := os.Stat(beside); err == nil {
			return beside
		}
	}
	// Fallback: assume it's in PATH
	return "claudestra"
}
