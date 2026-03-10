package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type WorkspaceConfig struct {
	Agents  []string `yaml:"agents"`
	Version string   `yaml:"version"`
}

type Workspace struct {
	Root         string
	OrchestraDir string
	IdeasDir     string
	LocksDir     string
	ContractsDir string
	JobsDir      string
	LogsDir      string
}

func NewWorkspace(root string) *Workspace {
	return &Workspace{
		Root:         root,
		OrchestraDir: filepath.Join(root, ".orchestra"),
		IdeasDir:     filepath.Join(root, ".orchestra", "ideas"),
		LocksDir:     filepath.Join(root, ".orchestra", "locks"),
		ContractsDir: filepath.Join(root, ".orchestra", "contracts"),
		JobsDir:      filepath.Join(root, ".orchestra", "jobs"),
		LogsDir:      filepath.Join(root, ".orchestra", "logs"),
	}
}

func (w *Workspace) Init(roles []string) error {
	os.MkdirAll(w.IdeasDir, 0755)
	os.MkdirAll(w.LocksDir, 0755)
	os.MkdirAll(w.ContractsDir, 0755)
	os.MkdirAll(w.JobsDir, 0755)
	os.MkdirAll(w.LogsDir, 0755)

	config := WorkspaceConfig{
		Agents:  roles,
		Version: "2.0",
	}
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(w.OrchestraDir, "config.yaml"), data, 0644); err != nil {
		return err
	}

	for _, role := range roles {
		agentDir := filepath.Join(w.Root, role)
		os.MkdirAll(agentDir, 0755)
	}
	return nil
}

func (w *Workspace) LoadConfig() (*WorkspaceConfig, error) {
	data, err := os.ReadFile(filepath.Join(w.OrchestraDir, "config.yaml"))
	if err != nil {
		return nil, err
	}
	var config WorkspaceConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// ── RolePlan Save/Load ──

func (w *Workspace) SaveRolePlans(plans []RolePlan) error {
	data, err := json.MarshalIndent(plans, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(w.OrchestraDir, "team.json"), data, 0644)
}

func (w *Workspace) LoadRolePlans() []RolePlan {
	data, err := os.ReadFile(filepath.Join(w.OrchestraDir, "team.json"))
	if err != nil {
		return nil
	}
	var plans []RolePlan
	if err := json.Unmarshal(data, &plans); err != nil {
		return nil
	}
	return plans
}

// ── Idea Management ──

func (w *Workspace) LoadIdea(role string) string {
	ideaFile := filepath.Join(w.IdeasDir, role+".yaml")
	data, err := os.ReadFile(ideaFile)
	if err != nil {
		return fmt.Sprintf("당신은 %s 전문가입니다.", role)
	}
	var doc map[string]string
	yaml.Unmarshal(data, &doc)
	if idea, ok := doc["idea"]; ok {
		return idea
	}
	return fmt.Sprintf("당신은 %s 전문가입니다.", role)
}

func (w *Workspace) SaveIdea(role, idea string) error {
	os.MkdirAll(w.IdeasDir, 0755)
	ideaFile := filepath.Join(w.IdeasDir, role+".yaml")
	data, err := yaml.Marshal(map[string]string{"role": role, "idea": idea})
	if err != nil {
		return err
	}
	return os.WriteFile(ideaFile, data, 0644)
}

// ── Contract ──

func (w *Workspace) SaveContract(content string) error {
	return os.WriteFile(filepath.Join(w.ContractsDir, "contract.yaml"), []byte(content), 0644)
}

func (w *Workspace) LoadContract() string {
	data, err := os.ReadFile(filepath.Join(w.ContractsDir, "contract.yaml"))
	if err != nil {
		return ""
	}
	return string(data)
}

// ── Session (direct access for CLI) ──

func (w *Workspace) SessionPath() string {
	return filepath.Join(w.OrchestraDir, "session.json")
}

func (w *Workspace) LoadSession() *Session {
	data, err := os.ReadFile(w.SessionPath())
	if err != nil {
		return &Session{}
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return &Session{}
	}
	return &s
}

func (w *Workspace) SaveSession(s *Session) error {
	os.MkdirAll(w.OrchestraDir, 0755)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(w.SessionPath(), data, 0644)
}

// ── Agent Factory (shared by GUI + CLI) ──

// BuildAgentsFromPlans creates an Agent map from RolePlan list.
// Same logic as app.go's buildTeamFromPlans but without Wails dependency.
func (w *Workspace) BuildAgentsFromPlans(plans []RolePlan) map[string]*Agent {
	lockRegistry := NewFileLockRegistry(w.LocksDir)
	agents := make(map[string]*Agent)

	var producerDirs []string
	for _, p := range plans {
		if p.Type == "producer" {
			producerDirs = append(producerDirs, filepath.Join(w.Root, p.Directory))
		}
	}

	contract := w.LoadContract()

	for _, plan := range plans {
		agentDir := filepath.Join(w.Root, plan.Directory)
		isConsumer := plan.Type == "consumer"

		var readRefs []string
		var allowedTools []string
		if isConsumer {
			readRefs = producerDirs
			allowedTools = ConsumerTools
		} else {
			allowedTools = ProducerTools
		}

		config := AgentConfig{
			AgentID:      plan.Role,
			Role:         plan.Role,
			Idea:         plan.Description,
			WorkDir:      agentDir,
			ReadRefs:     readRefs,
			AllowedTools: allowedTools,
			IsConsumer:   isConsumer,
			Contract:     contract,
			LogPath:      filepath.Join(w.LogsDir, plan.Role+".jsonl"),
		}
		agents[plan.Role] = NewAgent(config, lockRegistry)
	}
	return agents
}

// ── Workspace Discovery ──

// FindWorkspaceRoot walks up from the current directory looking for a .orchestra/ directory.
func FindWorkspaceRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".orchestra")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf(".orchestra 디렉토리를 찾을 수 없습니다")
}
