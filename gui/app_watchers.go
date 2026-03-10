package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gui/internal"

	"github.com/fsnotify/fsnotify"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// startLogWatcher creates and starts a LogWatcher for sub-agent JSONL output.
func (a *App) startLogWatcher(logFn func(string)) *internal.LogWatcher {
	var lastAgent, lastType string
	watcher := internal.NewLogWatcher(a.workspace.LogsDir, func(entry internal.LogEntry) {
		prefix := fmt.Sprintf("[%s] ", entry.Agent)
		isContinuation := entry.Agent == lastAgent && entry.Type == lastType &&
			(entry.Type == "thinking" || entry.Type == "text")

		switch entry.Type {
		case "thinking":
			if isContinuation {
				runtime.EventsEmit(a.ctx, "log-append", LogEvent{Message: entry.Message, Agent: entry.Agent})
			} else {
				runtime.EventsEmit(a.ctx, "log", LogEvent{Type: "thinking", Message: prefix + "💭 " + entry.Message, Agent: entry.Agent})
			}
		case "tool":
			runtime.EventsEmit(a.ctx, "log", LogEvent{Type: "thinking", Message: prefix + "🔧 " + entry.Message, Agent: entry.Agent})
		case "text":
			if isContinuation {
				runtime.EventsEmit(a.ctx, "log-append", LogEvent{Message: entry.Message, Agent: entry.Agent})
			} else {
				runtime.EventsEmit(a.ctx, "log", LogEvent{Type: "text", Message: prefix + entry.Message, Agent: entry.Agent})
			}
		case "status":
			runtime.EventsEmit(a.ctx, "log", LogEvent{Type: "text", Message: prefix + "📌 " + entry.Message, Agent: entry.Agent})
			runtime.EventsEmit(a.ctx, "team-updated", a.GetAgentStatuses())
		}
		lastAgent = entry.Agent
		lastType = entry.Type
	})
	if err := watcher.Start(); err != nil {
		logFn(fmt.Sprintf("[팀장] ⚠️ 로그 감시 시작 실패: %s", err))
	}
	return watcher
}

// startPermissionWatcher watches the permissions directory for GUI approval dialogs.
// Returns the watcher and a stop channel. Caller must close(stop) and watcher.Close() on cleanup.
func (a *App) startPermissionWatcher() (*fsnotify.Watcher, chan struct{}) {
	permDir := internal.PermissionsDir(a.workspace.Root)
	os.MkdirAll(permDir, 0755)
	permWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, nil
	}
	permWatcher.Add(permDir)
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			case event, ok := <-permWatcher.Events:
				if !ok {
					return
				}
				if (event.Has(fsnotify.Write) || event.Has(fsnotify.Create)) &&
					strings.Contains(event.Name, "request-") &&
					strings.HasSuffix(event.Name, ".json") {
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
	return permWatcher, stop
}

// startTeamWatcher watches team.json for changes and rebuilds the agent team.
// Returns the watcher and a stop channel. Caller must close(stop) and watcher.Close() on cleanup.
func (a *App) startTeamWatcher() (*fsnotify.Watcher, chan struct{}) {
	teamWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, nil
	}
	teamWatcher.Add(a.workspace.OrchestraDir)
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			case event, ok := <-teamWatcher.Events:
				if !ok {
					return
				}
				if (event.Has(fsnotify.Write) || event.Has(fsnotify.Create)) &&
					strings.HasSuffix(event.Name, "team.json") {
					if plans := a.workspace.LoadRolePlans(); len(plans) > 0 {
						a.rolePlans = plans
						a.rebuildTeam(plans)
						runtime.EventsEmit(a.ctx, "team-updated", a.GetAgentStatuses())
					}
				}
			case <-teamWatcher.Errors:
			}
		}
	}()
	return teamWatcher, stop
}
