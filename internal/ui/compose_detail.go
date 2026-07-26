package ui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cooffeeRequired/dockafe/internal/docker"
)

var (
	composeRowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	composeSelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("63")).
			Bold(true)
	composeMutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

func (m *Model) openComposeDetailLocal() {
	name := m.currentComposeName()
	if name == "" && m.composeProject != "" {
		name = m.composeProject
	}
	prevID := ""
	if svc, ok := m.selectedComposeService(); ok {
		prevID = svc.ID
	}
	m.composeProject = name
	m.syncComposeServices(prevID)
	m.detailTitle = "Compose · " + name
	m.mode = ModeComposeDetail
	m.returnToCompose = false
	m.status = "↑↓ select service · Enter inspect"
}

func (m *Model) syncComposeServices(preferID string) {
	m.composeServices = nil
	for _, g := range m.groups {
		if g.Name != m.composeProject {
			continue
		}
		m.composeServices = append(m.composeServices, g.Containers...)
		break
	}
	if len(m.composeServices) == 0 {
		m.composeCursor = 0
		return
	}
	if preferID != "" {
		for i, c := range m.composeServices {
			if c.ID == preferID {
				m.composeCursor = i
				return
			}
		}
	}
	if m.composeCursor < 0 {
		m.composeCursor = 0
	}
	if m.composeCursor >= len(m.composeServices) {
		m.composeCursor = len(m.composeServices) - 1
	}
}

func (m Model) composeGroup() (docker.ComposeGroup, bool) {
	for _, g := range m.groups {
		if g.Name == m.composeProject {
			return g, true
		}
	}
	return docker.ComposeGroup{}, false
}

func (m Model) selectedComposeService() (docker.ContainerInfo, bool) {
	if m.composeCursor < 0 || m.composeCursor >= len(m.composeServices) {
		return docker.ContainerInfo{}, false
	}
	return m.composeServices[m.composeCursor], true
}

func (m Model) viewComposeDetail() string {
	g, ok := m.composeGroup()
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		titleStyle.Render(" "+AppName+" "),
		"  ",
		activeTabStyle.Render(" COMPOSE "+AppVersion+" "),
		"  ",
		metaStyle.Render(m.detailTitle),
	)

	var b strings.Builder
	if ok {
		fmt.Fprintf(&b, "Compose project: %s\n", g.Name)
		fmt.Fprintf(&b, "Running: %d / %d    CPU: %s    MEM: %s\n", g.Running, g.Total, g.CPU, g.Mem)
		fmt.Fprintf(&b, "Ports: %s\n\n", g.Ports)
	} else {
		fmt.Fprintf(&b, "Compose project: %s\n(project not found — may have been removed)\n\n", m.composeProject)
	}

	b.WriteString(composeMutedStyle.Render("Services  (↑↓ select · Enter = inspect)"))
	b.WriteString("\n")

	nameW := 28
	stateW := 10
	cpuW := 7
	memW := 18
	for i, c := range m.composeServices {
		marker := "  "
		if i == m.composeCursor {
			marker = "▸ "
		}
		line := fmt.Sprintf("%s%-*s %-*s cpu=%-*s mem=%-*s  %s",
			marker,
			nameW, truncate(c.Name, nameW),
			stateW, c.State,
			cpuW, c.CPU,
			memW, truncate(c.Mem, memW),
			c.Ports,
		)
		if i == m.composeCursor {
			b.WriteString(composeSelStyle.Render(truncate(line, max(40, m.width-6))))
		} else {
			b.WriteString(composeRowStyle.Render(truncate(line, max(40, m.width-6))))
		}
		b.WriteByte('\n')
	}
	if len(m.composeServices) == 0 {
		b.WriteString(composeMutedStyle.Render("  (no services)"))
		b.WriteByte('\n')
	}

	body := panelStyle.Width(m.width - 2).Render(b.String())

	status := statusStyle.Render(m.status)
	if m.errMsg != "" {
		status = errorStyle.Render("ERR: " + truncate(m.errMsg, m.width-10))
	} else if svc, ok := m.selectedComposeService(); ok {
		status = statusStyle.Render(fmt.Sprintf("selected: %s (%s) · %s", svc.Name, short(svc.ID), svc.State))
	}

	help := helpFooterComposeDetail()
	if m.mode == ModeConfirm {
		help = confirmStyle.Render(m.confirmLabel + "  [y/n]")
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, status, help)
}

func (m Model) handleComposeDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.busy && key != "ctrl+c" {
		return m, nil
	}

	switch key {
	case "esc", "q", "backspace":
		m.mode = ModeList
		m.composeProject = ""
		m.returnToCompose = false
		m.relayout()
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "up":
		if m.composeCursor > 0 {
			m.composeCursor--
		}
		return m, nil
	case "down":
		if m.composeCursor < len(m.composeServices)-1 {
			m.composeCursor++
		}
		return m, nil
	case "enter", "i":
		return m.composeOpenInspect()
	case "l":
		return m.composeOpenLogs()
	case "t":
		return m.composeOpenTop()
	case "e":
		return m.composeExec()
	case "s":
		return m.composeStartService()
	case "x":
		return m.composeStopService()
	case "R":
		return m.composeRestartService()
	case "p":
		return m.composePauseService()
	case "k":
		return m.composeAskKill()
	case "d":
		return m.composeAskRemoveService()
	case "b":
		m.confirm = confirmRebuild
		m.confirmTarget = m.composeProject
		m.confirmLabel = fmt.Sprintf("Rebuild compose „%s“?", m.composeProject)
		m.mode = ModeConfirm
		return m, nil
	case "D":
		m.confirm = confirmRemoveAll
		m.confirmTarget = m.composeProject
		m.confirmLabel = fmt.Sprintf("Remove ALL compose containers “%s”?", m.composeProject)
		m.mode = ModeConfirm
		return m, nil
	case "c":
		m.selectedGroup = m.composeProject
		m.mode = ModeList
		m.tab = TabContainers
		m.ensureSortForTab()
		m.relayout()
		m.status = "project filter: " + m.composeProject
		return m, nil
	case "r":
		m.status = "refreshing…"
		return m, m.refresh(true)
	case "?":
		m.returnToCompose = true
		m.openHelp()
		return m, nil
	}
	return m, nil
}

func (m Model) composeOpenInspect() (tea.Model, tea.Cmd) {
	svc, ok := m.selectedComposeService()
	if !ok {
		return m, nil
	}
	m.returnToCompose = true
	m.targetID = svc.ID
	m.targetName = svc.Name
	m.status = "loading inspect…"
	return m, m.fetchTargetInspect()
}

func (m Model) composeOpenLogs() (tea.Model, tea.Cmd) {
	svc, ok := m.selectedComposeService()
	if !ok {
		m.status = "select a service with ↑↓"
		return m, nil
	}
	m.returnToCompose = true
	m.targetID = svc.ID
	m.targetName = svc.Name
	m.logsTarget = svc.ID
	m.logsName = svc.Name
	m.mode = ModeLogs
	m.logsFollow = true
	m.logsGen++
	m.detailTitle = "Logs · " + svc.Name
	m.vp.SetContent("loading logs…\n\n" + svc.Name + " (" + short(svc.ID) + ")")
	m.relayout()
	m.status = "logs: " + svc.Name
	return m, tea.Batch(m.fetchLogsGen(m.logsGen, m.logsTarget, m.logsName), logsTickCmd())
}

func (m Model) composeOpenTop() (tea.Model, tea.Cmd) {
	svc, ok := m.selectedComposeService()
	if !ok {
		return m, nil
	}
	m.returnToCompose = true
	id, name := svc.ID, svc.Name
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

func (m Model) composeExec() (tea.Model, tea.Cmd) {
	svc, ok := m.selectedComposeService()
	if !ok {
		return m, nil
	}
	cmd := exec.Command("docker", "exec", "-it", svc.ID, "sh", "-c", "command -v bash >/dev/null 2>&1 && exec bash || exec sh")
	m.status = "exec → " + svc.Name
	name := svc.Name
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{msg: "exec finished: " + name}
	})
}

func (m Model) composeStartService() (tea.Model, tea.Cmd) {
	svc, ok := m.selectedComposeService()
	if !ok {
		return m, nil
	}
	m.busy = true
	m.status = "starting " + svc.Name
	id, name := svc.ID, svc.Name
	return m, m.runAction(func(ctx context.Context) error {
		return m.client.StartContainer(ctx, id)
	}, "started: "+name)
}

func (m Model) composeStopService() (tea.Model, tea.Cmd) {
	svc, ok := m.selectedComposeService()
	if !ok {
		return m, nil
	}
	m.busy = true
	m.status = "stopping " + svc.Name
	id, name := svc.ID, svc.Name
	return m, m.runAction(func(ctx context.Context) error {
		return m.client.StopContainer(ctx, id)
	}, "stopped: "+name)
}

func (m Model) composeRestartService() (tea.Model, tea.Cmd) {
	svc, ok := m.selectedComposeService()
	if !ok {
		return m, nil
	}
	m.busy = true
	m.status = "restarting " + svc.Name
	id, name := svc.ID, svc.Name
	return m, m.runAction(func(ctx context.Context) error {
		return m.client.RestartContainer(ctx, id)
	}, "restarted: "+name)
}

func (m Model) composePauseService() (tea.Model, tea.Cmd) {
	svc, ok := m.selectedComposeService()
	if !ok {
		return m, nil
	}
	m.busy = true
	id, name := svc.ID, svc.Name
	if svc.State == "paused" {
		m.status = "unpause " + name
		return m, m.runAction(func(ctx context.Context) error {
			return m.client.UnpauseContainer(ctx, id)
		}, "unpaused: "+name)
	}
	m.status = "pause " + name
	return m, m.runAction(func(ctx context.Context) error {
		return m.client.PauseContainer(ctx, id)
	}, "paused: "+name)
}

func (m Model) composeAskKill() (tea.Model, tea.Cmd) {
	svc, ok := m.selectedComposeService()
	if !ok {
		return m, nil
	}
	m.confirm = confirmKill
	m.confirmTarget = svc.ID + "|" + svc.Name
	m.confirmLabel = fmt.Sprintf("KILL (SIGKILL) „%s“?", svc.Name)
	m.mode = ModeConfirm
	return m, nil
}

func (m Model) composeAskRemoveService() (tea.Model, tea.Cmd) {
	svc, ok := m.selectedComposeService()
	if !ok {
		return m, nil
	}
	m.confirm = confirmRemove
	m.confirmTarget = svc.ID + "|" + svc.Name
	m.confirmLabel = fmt.Sprintf("Remove container “%s”?", svc.Name)
	m.mode = ModeConfirm
	return m, nil
}

func (m *Model) backFromPanel() {
	m.logsFollow = false
	if m.returnToCompose && m.composeProject != "" {
		m.returnToCompose = false
		m.mode = ModeComposeDetail
		m.detailTitle = "Compose · " + m.composeProject
		m.syncComposeServices("")
		return
	}
	m.mode = ModeList
}
