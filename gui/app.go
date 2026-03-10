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

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// LogEvent is a structured log entry sent to the frontend.
type LogEvent struct {
	Type    string `json:"type"`    // "text", "thinking", "status"
	Message string `json:"message"`
	Agent   string `json:"agent"`   // agent name (empty = lead)
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
	ws := internal.NewWorkspace(projectDir)
	a.workspace = ws
	a.lead = internal.NewLeadAgent(projectDir)
	a.agents = make(map[string]*internal.Agent)

	// 기존 팀이 있으면 복원
	a.rolePlans = ws.LoadRolePlans()
	if len(a.rolePlans) > 0 {
		a.rebuildTeam(a.rolePlans)
	}
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
		a.rebuildTeam(a.rolePlans)
	} else if len(config.Agents) > 0 {
		// Legacy compat: config.yaml with only roles
		a.buildLegacyTeam(config.Agents)
	}
	return nil
}

// rebuildTeam builds the agent team from plans using the shared factory.
func (a *App) rebuildTeam(plans []internal.RolePlan) {
	agents := a.workspace.BuildAgentsFromPlans(plans, internal.BuildOptions{DetectWriteTool: true, LoadContract: true})
	a.agents = agents
	a.lead = internal.NewLeadAgent(a.workspace.Root)
	for _, agent := range agents {
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
	a.rebuildTeam(plans)
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

// ── Single Session Mode (Phase D+E) ──

// RunLeadSession runs the entire workflow in a single Claude session via the lead agent.
// It watches .orchestra/logs/ with fsnotify to stream sub-agent JSONL output in real time.
func (a *App) RunLeadSession(userInput string) string {
	if a.lead == nil {
		return "프로젝트를 먼저 열어주세요."
	}

	logFn := func(msg string) {
		if len(msg) > 0 && msg[0] == '\x01' {
			runtime.EventsEmit(a.ctx, "log-append", LogEvent{Message: msg[1:], Agent: ""})
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

	// Start watchers (log, permission, team)
	logWatcher := a.startLogWatcher(logFn)
	defer logWatcher.Stop()

	permWatcher, permStop := a.startPermissionWatcher()
	if permWatcher != nil {
		defer func() {
			close(permStop)
			permWatcher.Close()
		}()
	}

	teamWatcher, teamStop := a.startTeamWatcher()
	if teamWatcher != nil {
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
		a.rebuildTeam(plans)
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
