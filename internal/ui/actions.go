package ui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) currentComposeName() string {
	if m.mode == ModeMultiHost {
		return m.multiSelectedCompose()
	}
	rows := m.table.Rows()
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(rows) {
		return ""
	}
	return rows[idx][0]
}

func (m Model) currentContainer() (id, name string) {
	if m.mode == ModeMultiHost {
		return m.multiSelectedContainer()
	}
	idx := m.table.Cursor()
	list := m.filteredContainers()
	if idx < 0 || idx >= len(list) {
		return "", ""
	}
	return list[idx].ID, list[idx].Name
}

func (m Model) currentImageID() string {
	idx := m.table.Cursor()
	list := m.filteredImages()
	if idx < 0 || idx >= len(list) {
		return ""
	}
	if list[idx].FullID != "" {
		return list[idx].FullID
	}
	return list[idx].ID
}

func (m Model) currentVolumeName() string {
	idx := m.table.Cursor()
	list := m.filteredVolumes()
	if idx < 0 || idx >= len(list) {
		return ""
	}
	return list[idx].Name
}

func (m Model) currentNetwork() (id, name string) {
	idx := m.table.Cursor()
	list := m.filteredNetworks()
	if idx < 0 || idx >= len(list) {
		return "", ""
	}
	return list[idx].ID, list[idx].Name
}

func (m Model) startSelected() (tea.Model, tea.Cmd) {
	if m2, ok := m.guardMutate(); !ok {
		return m2, nil
	}
	switch m.tab {
	case TabCompose:
		name := m.currentComposeName()
		if name == "" {
			return m, nil
		}
		m.busy = true
		m.status = "starting compose " + name
		return m, m.runAction(func(ctx context.Context) error {
			return m.focusedClient().StartCompose(ctx, name)
		}, "compose started: "+name)
	case TabContainers:
		id, name := m.currentContainer()
		if id == "" {
			return m, nil
		}
		m.busy = true
		m.status = "starting " + name
		return m, m.runAction(func(ctx context.Context) error {
			return m.focusedClient().StartContainer(ctx, id)
		}, "started: "+name)
	}
	return m, nil
}

func (m Model) stopSelected() (tea.Model, tea.Cmd) {
	if m2, ok := m.guardMutate(); !ok {
		return m2, nil
	}
	switch m.tab {
	case TabCompose:
		name := m.currentComposeName()
		if name == "" {
			return m, nil
		}
		m.busy = true
		m.status = "stopping compose " + name
		return m, m.runAction(func(ctx context.Context) error {
			return m.focusedClient().StopCompose(ctx, name)
		}, "compose stopped: "+name)
	case TabContainers:
		id, name := m.currentContainer()
		if id == "" {
			return m, nil
		}
		m.busy = true
		m.status = "stopping " + name
		return m, m.runAction(func(ctx context.Context) error {
			return m.focusedClient().StopContainer(ctx, id)
		}, "stopped: "+name)
	}
	return m, nil
}

func (m Model) restartSelected() (tea.Model, tea.Cmd) {
	if m2, ok := m.guardMutate(); !ok {
		return m2, nil
	}
	switch m.tab {
	case TabCompose:
		name := m.currentComposeName()
		if name == "" {
			return m, nil
		}
		m.busy = true
		m.status = "restarting compose " + name
		return m, m.runAction(func(ctx context.Context) error {
			return m.focusedClient().RestartCompose(ctx, name)
		}, "compose restarted: "+name)
	case TabContainers:
		id, name := m.currentContainer()
		if id == "" {
			return m, nil
		}
		m.busy = true
		m.status = "restarting " + name
		return m, m.runAction(func(ctx context.Context) error {
			return m.focusedClient().RestartContainer(ctx, id)
		}, "restarted: "+name)
	}
	return m, nil
}

func (m Model) pauseSelected() (tea.Model, tea.Cmd) {
	if m2, ok := m.guardMutate(); !ok {
		return m2, nil
	}
	if m.tab != TabContainers {
		m.status = "pause only on Containers"
		return m, nil
	}
	id, name := m.currentContainer()
	if id == "" {
		return m, nil
	}
	list := m.filteredContainers()
	idx := m.table.Cursor()
	paused := idx >= 0 && idx < len(list) && list[idx].State == "paused"
	m.busy = true
	if paused {
		m.status = "unpause " + name
		return m, m.runAction(func(ctx context.Context) error {
			return m.focusedClient().UnpauseContainer(ctx, id)
		}, "unpaused: "+name)
	}
	m.status = "pause " + name
	return m, m.runAction(func(ctx context.Context) error {
		return m.focusedClient().PauseContainer(ctx, id)
	}, "paused: "+name)
}

func (m Model) askRebuild() (tea.Model, tea.Cmd) {
	if m2, ok := m.guardMutate(); !ok {
		return m2, nil
	}
	if m.tab != TabCompose {
		m.status = "rebuild only on Compose"
		return m, nil
	}
	name := m.currentComposeName()
	if name == "" || name == "(standalone)" {
		m.status = "select a compose project"
		return m, nil
	}
	m.confirm = confirmRebuild
	m.confirmTarget = name
	m.confirmLabel = fmt.Sprintf("Rebuild compose „%s“?", name)
	m.mode = ModeConfirm
	return m, nil
}

func (m Model) askRemove() (tea.Model, tea.Cmd) {
	if m2, ok := m.guardMutate(); !ok {
		return m2, nil
	}
	switch m.tab {
	case TabCompose:
		name := m.currentComposeName()
		if name == "" {
			return m, nil
		}
		m.confirm = confirmRemoveAll
		m.confirmTarget = name
		m.confirmLabel = fmt.Sprintf("Remove ALL compose containers “%s”?", name)
	case TabContainers:
		id, name := m.currentContainer()
		if id == "" {
			return m, nil
		}
		m.confirm = confirmRemove
		m.confirmTarget = id + "|" + name
		m.confirmLabel = fmt.Sprintf("Remove container “%s”?", name)
	case TabImages:
		id := m.currentImageID()
		if id == "" {
			return m, nil
		}
		m.confirm = confirmRemove
		m.confirmTarget = "image|" + id
		m.confirmLabel = fmt.Sprintf("Remove image “%s”?", short(id))
	case TabVolumes:
		name := m.currentVolumeName()
		if name == "" {
			return m, nil
		}
		m.confirm = confirmRemove
		m.confirmTarget = "volume|" + name
		m.confirmLabel = fmt.Sprintf("Remove volume “%s”?", name)
	case TabNetworks:
		id, name := m.currentNetwork()
		if id == "" {
			return m, nil
		}
		m.confirm = confirmRemove
		m.confirmTarget = "network|" + id + "|" + name
		m.confirmLabel = fmt.Sprintf("Remove network “%s”?", name)
	}
	m.mode = ModeConfirm
	return m, nil
}

func (m Model) askRemoveAll() (tea.Model, tea.Cmd) {
	if m2, ok := m.guardMutate(); !ok {
		return m2, nil
	}
	if m.tab != TabCompose {
		m.status = "Remove All (D) only on Compose"
		return m, nil
	}
	name := m.currentComposeName()
	if name == "" {
		return m, nil
	}
	m.confirm = confirmRemoveAll
	m.confirmTarget = name
	m.confirmLabel = fmt.Sprintf("Remove ALL compose containers “%s”?", name)
	m.mode = ModeConfirm
	return m, nil
}

func (m Model) askKill() (tea.Model, tea.Cmd) {
	if m2, ok := m.guardMutate(); !ok {
		return m2, nil
	}
	if m.tab != TabContainers {
		m.status = "kill only on Containers"
		return m, nil
	}
	id, name := m.currentContainer()
	if id == "" {
		return m, nil
	}
	m.confirm = confirmKill
	m.confirmTarget = id + "|" + name
	m.confirmLabel = fmt.Sprintf("KILL (SIGKILL) „%s“?", name)
	m.mode = ModeConfirm
	return m, nil
}

func (m Model) askPrune() (tea.Model, tea.Cmd) {
	if m2, ok := m.guardMutate(); !ok {
		return m2, nil
	}
	var target string
	switch m.tab {
	case TabContainers:
		target = "containers"
		m.confirmLabel = "Prune stopped containers?"
	case TabImages:
		target = "images"
		m.confirmLabel = "Prune unused images?"
	case TabVolumes:
		target = "volumes"
		m.confirmLabel = "Prune unused volumes?"
	case TabNetworks:
		target = "networks"
		m.confirmLabel = "Prune unused networks?"
	default:
		m.status = "Prune (P) on Containers/Images/Volumes/Networks"
		return m, nil
	}
	m.confirm = confirmPrune
	m.confirmTarget = target
	m.mode = ModeConfirm
	return m, nil
}

func (m Model) doRebuild(project string) (tea.Model, tea.Cmd) {
	m.busy = true
	m.status = "rebuild " + project
	return m, m.runAction(func(ctx context.Context) error {
		return m.focusedClient().RebuildCompose(ctx, project)
	}, "rebuild done: "+project)
}

func (m Model) doRemove(target string) (tea.Model, tea.Cmd) {
	m.busy = true
	parts := strings.SplitN(target, "|", 3)
	switch {
	case len(parts) == 2 && parts[0] == "image":
		id := parts[1]
		m.status = "removing image " + short(id)
		return m, m.runAction(func(ctx context.Context) error {
			return m.focusedClient().RemoveImage(ctx, id, true)
		}, "image removed: "+short(id))
	case len(parts) == 2 && parts[0] == "volume":
		name := parts[1]
		m.status = "removing volume " + name
		return m, m.runAction(func(ctx context.Context) error {
			return m.focusedClient().RemoveVolume(ctx, name, true)
		}, "volume removed: "+name)
	case len(parts) == 3 && parts[0] == "network":
		id, name := parts[1], parts[2]
		m.status = "removing network " + name
		return m, m.runAction(func(ctx context.Context) error {
			return m.focusedClient().RemoveNetwork(ctx, id)
		}, "network removed: "+name)
	default:
		id, name := parts[0], parts[0]
		if len(parts) > 1 {
			name = parts[1]
		}
		m.status = "removing " + name
		return m, m.runAction(func(ctx context.Context) error {
			_ = m.focusedClient().StopContainer(ctx, id)
			return m.focusedClient().RemoveContainer(ctx, id, true)
		}, "removed: "+name)
	}
}

func (m Model) doRemoveAll(project string) (tea.Model, tea.Cmd) {
	m.busy = true
	m.status = "removing compose " + project
	return m, m.runAction(func(ctx context.Context) error {
		return m.focusedClient().RemoveCompose(ctx, project, true)
	}, "compose removed: "+project)
}

func (m Model) doKill(target string) (tea.Model, tea.Cmd) {
	parts := strings.SplitN(target, "|", 2)
	id, name := parts[0], parts[0]
	if len(parts) > 1 {
		name = parts[1]
	}
	m.busy = true
	m.status = "kill " + name
	return m, m.runAction(func(ctx context.Context) error {
		return m.focusedClient().KillContainer(ctx, id)
	}, "killed: "+name)
}

func (m Model) doPrune(kind string) (tea.Model, tea.Cmd) {
	m.busy = true
	m.status = "prune " + kind
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		var msg string
		var err error
		switch kind {
		case "images":
			msg, err = m.focusedClient().PruneImages(ctx)
		case "volumes":
			msg, err = m.focusedClient().PruneVolumes(ctx)
		case "networks":
			msg, err = m.focusedClient().PruneNetworks(ctx)
		default:
			msg, err = m.focusedClient().PruneContainers(ctx)
		}
		return actionDoneMsg{err: err, msg: msg}
	}
}

func (m Model) execShell() (tea.Model, tea.Cmd) {
	if m.tab != TabContainers {
		m.status = "exec only on Containers"
		return m, nil
	}
	id, name := m.currentContainer()
	if id == "" {
		return m, nil
	}
	cmd := exec.Command("docker", "exec", "-it", id, "sh", "-c", "command -v bash >/dev/null 2>&1 && exec bash || exec sh")
	m.status = "exec → " + name
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return actionDoneMsg{err: err, msg: ""}
		}
		return actionDoneMsg{msg: "exec finished: " + name}
	})
}
