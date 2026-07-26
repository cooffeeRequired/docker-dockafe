package ui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// resolveTarget fills targetID/Name from current list selection when empty.
func (m *Model) resolveTarget() bool {
	if m.targetID != "" {
		return true
	}
	// Fallback: match by detail title "Container · name"
	if strings.HasPrefix(m.detailTitle, "Container · ") {
		name := strings.TrimPrefix(m.detailTitle, "Container · ")
		for _, c := range m.containers {
			if c.Name == name {
				m.targetID = c.ID
				m.targetName = c.Name
				return true
			}
		}
	}
	switch m.tab {
	case TabCompose:
		if m.mode == ModeComposeDetail {
			svc, ok := m.selectedComposeService()
			if !ok {
				return false
			}
			m.targetID = svc.ID
			m.targetName = svc.Name
			return true
		}
		name := m.currentComposeName()
		if name == "" && m.composeProject != "" {
			name = m.composeProject
		}
		for _, g := range m.groups {
			if g.Name != name {
				continue
			}
			for _, c := range g.Containers {
				if c.Running {
					m.targetID = c.ID
					m.targetName = c.Name
					return true
				}
			}
			if len(g.Containers) > 0 {
				m.targetID = g.Containers[0].ID
				m.targetName = g.Containers[0].Name
				return true
			}
		}
	case TabContainers:
		id, name := m.currentContainer()
		if id == "" {
			return false
		}
		m.targetID = id
		m.targetName = name
		return true
	}
	return false
}

func (m Model) openLogsForTarget() (tea.Model, tea.Cmd) {
	// From compose list without interactive detail: show all project logs
	if m.tab == TabCompose && m.mode == ModeList {
		return m.openLogs()
	}
	if m.tab == TabCompose && m.mode == ModeComposeDetail {
		return m.composeOpenLogs()
	}

	if !m.resolveTarget() {
		m.status = "no container selected for logs"
		m.errMsg = "cannot open logs — missing target container"
		return m, nil
	}

	m.logsTarget = m.targetID
	m.logsName = m.targetName
	m.mode = ModeLogs
	m.logsFollow = true
	m.logsGen++
	m.detailTitle = "Logs · " + m.targetName
	m.vp.SetContent("loading logs…\n\n" + m.targetName + "\n" + m.targetID)
	m.relayout()
	m.status = "loading logs: " + m.targetName
	m.errMsg = ""
	return m, tea.Batch(m.fetchLogsGen(m.logsGen, m.logsTarget, m.logsName), logsTickCmd())
}

func (m Model) openTopForTarget() (tea.Model, tea.Cmd) {
	if m.mode == ModeComposeDetail {
		return m.composeOpenTop()
	}
	if !m.resolveTarget() {
		m.status = "no container selected for top"
		return m, nil
	}
	id, name := m.targetID, m.targetName
	m.returnToCompose = m.composeProject != ""
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		body, err := m.client.ContainerTop(ctx, id)
		if err != nil {
			return contentMsg{err: err, mode: ModeDetail}
		}
		return contentMsg{title: "Top · " + name, body: body, mode: ModeDetail}
	}
}

func (m Model) execTarget() (tea.Model, tea.Cmd) {
	if m.mode == ModeComposeDetail {
		return m.composeExec()
	}
	if !m.resolveTarget() {
		m.status = "no container selected for exec"
		return m, nil
	}
	id, name := m.targetID, m.targetName
	cmd := exec.Command("docker", "exec", "-it", id, "sh", "-c", "command -v bash >/dev/null 2>&1 && exec bash || exec sh")
	m.status = "exec → " + name
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return actionDoneMsg{err: fmt.Errorf("exec %s: %w", name, err)}
		}
		return actionDoneMsg{msg: "exec finished: " + name}
	})
}

func (m Model) actionOnTarget(action string) (tea.Model, tea.Cmd) {
	if m.mode == ModeComposeDetail {
		switch action {
		case "start":
			return m.composeStartService()
		case "stop":
			return m.composeStopService()
		case "restart":
			return m.composeRestartService()
		}
	}
	if !m.resolveTarget() {
		m.status = "no container selected"
		return m, nil
	}
	id, name := m.targetID, m.targetName
	m.busy = true
	switch action {
	case "start":
		m.status = "starting " + name
		return m, m.runAction(func(ctx context.Context) error {
			return m.client.StartContainer(ctx, id)
		}, "started: "+name)
	case "stop":
		m.status = "stopping " + name
		return m, m.runAction(func(ctx context.Context) error {
			return m.client.StopContainer(ctx, id)
		}, "stopped: "+name)
	case "restart":
		m.status = "restarting " + name
		return m, m.runAction(func(ctx context.Context) error {
			return m.client.RestartContainer(ctx, id)
		}, "restarted: "+name)
	}
	m.busy = false
	return m, nil
}

func (m Model) fetchTargetInspect() tea.Cmd {
	id, name := m.targetID, m.targetName
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		inspect, err := m.client.InspectContainer(ctx, id)
		if err != nil {
			return contentMsg{err: err, mode: ModeDetail, targetID: id, targetName: name}
		}
		top, _ := m.client.ContainerTop(ctx, id)
		body := fmt.Sprintf(
			"[ l / f = logs ]  [ e = exec ]  [ t = top ]  [ esc = back ]\n\nName: %s\nID: %s\n\n=== PROCESSES ===\n%s\n\n=== INSPECT ===\n%s\n",
			name, id, emptyDash(top), inspect,
		)
		return contentMsg{title: "Container · " + name, body: body, mode: ModeDetail, targetID: id, targetName: name}
	}
}

func (m Model) reloadLogs() tea.Cmd {
	// Caller must increment logsGen before calling when starting a new fetch.
	if strings.Contains(m.logsTarget, ",") {
		ids := strings.Split(m.logsTarget, ",")
		names := make([]string, len(ids))
		for i, id := range ids {
			names[i] = short(id)
			for _, c := range m.containers {
				if c.ID == id {
					names[i] = c.Name
					break
				}
			}
		}
		return m.fetchComposeLogsGen(m.logsGen, ids, names)
	}
	return m.fetchLogsGen(m.logsGen, m.logsTarget, m.logsName)
}
