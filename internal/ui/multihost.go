package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cooffeeRequired/dockafe/internal/docker"
)

type hostPickTarget int

const (
	hostPickLeft hostPickTarget = iota
	hostPickRight
)

func (m Model) focusedClient() *docker.Client {
	pane := m.actionPane
	if m.mode == ModeMultiHost {
		pane = m.multiFocus
	}
	if pane == 1 && m.clientRight != nil {
		return m.clientRight
	}
	return m.client
}

func (m Model) actionGroups() []docker.ComposeGroup {
	if m.actionPane == 1 || (m.mode == ModeMultiHost && m.multiFocus == 1) {
		return m.groupsRight
	}
	return m.groups
}

func (m Model) actionContainers() []docker.ContainerInfo {
	if m.actionPane == 1 || (m.mode == ModeMultiHost && m.multiFocus == 1) {
		return m.containersRight
	}
	return m.containers
}

func (m Model) openMultiHost() (tea.Model, tea.Cmd) {
	if m.client != nil && m.client.IsDemo() {
		m.status = "multi-host disabled in demo mode"
		return m, nil
	}
	if m.tab != TabCompose && m.tab != TabContainers {
		m.tab = TabCompose
	}
	if m.clientRight == nil {
		m.hostPickTarget = hostPickRight
		m.status = "pick right-hand Docker host for side-by-side"
		return m.openHosts()
	}
	m.mode = ModeMultiHost
	m.multiFocus = 0
	m.errMsg = ""
	m.status = "multi-host · Tab focus · H change focused host · esc back"
	m.relayout()
	var cmds []tea.Cmd
	if !m.loadingRight {
		m.dataGenRight++
		m.loadingRight = true
		cmds = append(cmds, m.refreshPaneCmd(1, false, m.dataGenRight))
	}
	return m, tea.Batch(cmds...)
}

func (m Model) leaveMultiHost() (tea.Model, tea.Cmd) {
	m.mode = ModeList
	m.relayout()
	m.applyRows()
	m.status = "single host · " + m.dockerHostLabel()
	return m, nil
}

func (m Model) multiGroups(focus int) []docker.ComposeGroup {
	if focus == 1 {
		return m.groupsRight
	}
	return m.groups
}

func (m Model) multiContainers(focus int) []docker.ContainerInfo {
	if focus == 1 {
		return m.containersRight
	}
	return m.containers
}

func (m Model) multiCursor(focus int) int {
	if focus == 1 {
		return m.multiCursorR
	}
	return m.multiCursorL
}

func (m *Model) setMultiCursor(focus, cur int) {
	if focus == 1 {
		m.multiCursorR = cur
	} else {
		m.multiCursorL = cur
	}
}

func (m Model) multiSelectedCompose() string {
	groups := m.multiGroups(m.multiFocus)
	cur := m.multiCursor(m.multiFocus)
	list := m.filterComposeGroups(groups)
	if cur < 0 || cur >= len(list) {
		return ""
	}
	return list[cur].Name
}

func (m Model) multiSelectedContainer() (id, name string) {
	list := m.filterContainersList(m.multiContainers(m.multiFocus))
	cur := m.multiCursor(m.multiFocus)
	if cur < 0 || cur >= len(list) {
		return "", ""
	}
	return list[cur].ID, list[cur].Name
}

func (m Model) filterComposeGroups(groups []docker.ComposeGroup) []docker.ComposeGroup {
	q := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	out := make([]docker.ComposeGroup, 0, len(groups))
	for _, g := range groups {
		if m.runningOnly && g.Running == 0 {
			continue
		}
		if q != "" && !matchesFilter(q, g.Name, g.Ports, g.CPU, g.Mem) {
			continue
		}
		out = append(out, g)
	}
	return out
}

func (m Model) filterContainersList(containers []docker.ContainerInfo) []docker.ContainerInfo {
	q := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	out := make([]docker.ContainerInfo, 0, len(containers))
	for _, c := range containers {
		if m.runningOnly && !c.Running {
			continue
		}
		if m.selectedGroup != "" && c.Project != m.selectedGroup {
			continue
		}
		if q != "" && !matchesFilter(q, c.Name, c.Image, c.Ports, c.Project, c.State, c.Status) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func (m Model) viewMultiHost() string {
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		titleStyle.Render(m.versionTitle()),
		"  ",
		activeTabStyle.Render(" MULTI "),
		"  ",
		metaStyle.Render(tabNames[m.tab]+" · side-by-side"),
	)
	if badge := m.updateBadge(); badge != "" {
		header = lipgloss.JoinHorizontal(lipgloss.Top, header, " ", badge)
	}

	colW := max(28, (m.width-3)/2)
	bodyH := max(8, m.height-8)
	left := m.renderHostColumn(0, colW, bodyH)
	right := m.renderHostColumn(1, colW, bodyH)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)

	status := statusStyle.Render(m.status)
	if m.errMsg != "" {
		status = errorStyle.Render("ERR: " + truncate(m.errMsg, m.width-10))
	}
	help := renderHelpRow("multi", []helpBinding{
		{"Tab", "focus"},
		{"↑↓", "move"},
		{"Enter", "open"},
		{"H", "host"},
		{"s/x/R", "lifecycle"},
		{"g", "graphs"},
		{"l", "logs"},
		{"esc", "back"},
	})
	return lipgloss.JoinVertical(lipgloss.Left, header, body, status, help)
}

func (m Model) renderHostColumn(focus, width, height int) string {
	border := cBorder
	if m.multiFocus == focus {
		border = cActiveBg
	}
	title := "LEFT"
	host := m.dockerHostLabel()
	sys := m.sysInfo
	err := ""
	loading := m.loading
	if focus == 1 {
		title = "RIGHT"
		host = "-"
		if m.clientRight != nil {
			host = m.clientRight.Host()
		}
		sys = m.sysInfoRight
		err = m.errRight
		loading = m.loadingRight
	}
	var b strings.Builder
	b.WriteString(helpTitleStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(metaStyle.Render(truncate(host, width-4)))
	b.WriteString("\n")
	if err != "" {
		b.WriteString(errorStyle.Render(truncate(err, width-4)))
		b.WriteString("\n")
	} else if loading {
		b.WriteString(chartLabelStyle().Render("loading…"))
		b.WriteString("\n")
	} else if sys != "" {
		b.WriteString(chartLabelStyle().Render(truncate(sys, width-4)))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(m.renderMultiRows(focus, width-4, height-6))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Width(width).
		Height(height).
		Padding(0, 1).
		Render(b.String())
}

func (m Model) renderMultiRows(focus, width, maxRows int) string {
	cur := m.multiCursor(focus)
	var lines []string
	switch m.tab {
	case TabContainers:
		list := m.filterContainersList(m.multiContainers(focus))
		if cur >= len(list) && len(list) > 0 {
			cur = len(list) - 1
		}
		for i, c := range list {
			if i >= maxRows {
				lines = append(lines, chartMutedStyle().Render(fmt.Sprintf("… +%d more", len(list)-maxRows)))
				break
			}
			line := fmt.Sprintf("%-18s %-10s %6s %10s",
				truncate(c.Name, 18),
				truncate(c.State, 10),
				truncate(c.CPU, 6),
				truncate(c.Mem, 10),
			)
			if i == cur {
				lines = append(lines, activeTabStyle.Render(truncate(line, width)))
			} else {
				lines = append(lines, helpDescStyle.Render(truncate(line, width)))
			}
		}
		if len(list) == 0 {
			lines = append(lines, chartMutedStyle().Render("no containers"))
		}
	default:
		list := m.filterComposeGroups(m.multiGroups(focus))
		if cur >= len(list) && len(list) > 0 {
			cur = len(list) - 1
		}
		for i, g := range list {
			if i >= maxRows {
				lines = append(lines, chartMutedStyle().Render(fmt.Sprintf("… +%d more", len(list)-maxRows)))
				break
			}
			up := fmt.Sprintf("%d/%d", g.Running, g.Total)
			line := fmt.Sprintf("%-16s %5s %6s %10s",
				truncate(g.Name, 16),
				up,
				truncate(g.CPU, 6),
				truncate(g.Mem, 10),
			)
			if i == cur {
				lines = append(lines, activeTabStyle.Render(truncate(line, width)))
			} else {
				lines = append(lines, helpDescStyle.Render(truncate(line, width)))
			}
		}
		if len(list) == 0 {
			lines = append(lines, chartMutedStyle().Render("no compose projects"))
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) handleMultiHostKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc", "q":
		return m.leaveMultiHost()
	case "ctrl+c":
		return m, tea.Quit
	case "tab":
		if m.clientRight == nil {
			m.status = "pick a right host first (H)"
			return m, nil
		}
		m.multiFocus = 1 - m.multiFocus
		m.status = fmt.Sprintf("focus %s", map[int]string{0: "LEFT", 1: "RIGHT"}[m.multiFocus])
		return m, nil
	case "H":
		if m.multiFocus == 1 {
			m.hostPickTarget = hostPickRight
		} else {
			m.hostPickTarget = hostPickLeft
		}
		return m.openHosts()
	case "M":
		return m.leaveMultiHost()
	case "1":
		m.tab = TabCompose
		m.clampMultiCursors()
		return m, nil
	case "2":
		m.tab = TabContainers
		m.clampMultiCursors()
		return m, nil
	case "up", "k":
		cur := m.multiCursor(m.multiFocus)
		if cur > 0 {
			m.setMultiCursor(m.multiFocus, cur-1)
		}
		return m, nil
	case "down", "j":
		maxIdx := m.multiRowCount(m.multiFocus) - 1
		cur := m.multiCursor(m.multiFocus)
		if cur < maxIdx {
			m.setMultiCursor(m.multiFocus, cur+1)
		}
		return m, nil
	case "enter":
		return m.openMultiSelection()
	case "g":
		m.actionPane = m.multiFocus
		m.returnToMulti = true
		return m.openGraphs()
	case "l", "f":
		m.actionPane = m.multiFocus
		m.returnToMulti = true
		return m.openLogsForTarget()
	case "s":
		m.actionPane = m.multiFocus
		return m.startSelected()
	case "x":
		m.actionPane = m.multiFocus
		return m.stopSelected()
	case "R":
		m.actionPane = m.multiFocus
		return m.restartSelected()
	}
	return m, nil
}

func (m Model) multiRowCount(focus int) int {
	if m.tab == TabContainers {
		return len(m.filterContainersList(m.multiContainers(focus)))
	}
	return len(m.filterComposeGroups(m.multiGroups(focus)))
}

func (m *Model) clampMultiCursors() {
	for _, focus := range []int{0, 1} {
		n := m.multiRowCount(focus)
		cur := m.multiCursor(focus)
		if n == 0 {
			m.setMultiCursor(focus, 0)
			continue
		}
		if cur >= n {
			m.setMultiCursor(focus, n-1)
		}
	}
}

func (m Model) openMultiSelection() (tea.Model, tea.Cmd) {
	m.actionPane = m.multiFocus
	m.returnToMulti = true
	switch m.tab {
	case TabCompose:
		name := m.multiSelectedCompose()
		if name == "" {
			return m, nil
		}
		m.composeProject = name
		m.returnToCompose = true
		m.mode = ModeComposeDetail
		m.syncComposeServices("")
		m.status = "compose · " + name + " · " + m.focusedClient().Host()
		return m, nil
	case TabContainers:
		id, name := m.multiSelectedContainer()
		if id == "" {
			return m, nil
		}
		m.targetID = id
		m.targetName = name
		m.status = "loading detail…"
		return m, m.fetchTargetInspect()
	}
	return m, nil
}

// refreshPaneCmd refreshes inventory for pane 0 (left) or 1 (right).
func (m Model) refreshPaneCmd(pane int, withStats bool, gen uint64) tea.Cmd {
	var client *docker.Client
	if pane == 1 {
		client = m.clientRight
	} else {
		client = m.client
	}
	return func() tea.Msg {
		if client == nil {
			return dataMsg{gen: gen, pane: pane, err: fmt.Errorf("no client"), at: time.Now()}
		}
		timeout := 12 * time.Second
		remote := client.IsRemoteDaemon()
		if remote {
			timeout = 25 * time.Second
		} else if withStats {
			timeout = 20 * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		sys, _ := client.SystemInfo(ctx)
		stats := withStats && !remote
		containers, err := client.ListContainers(ctx, stats)
		if err != nil {
			return dataMsg{gen: gen, pane: pane, err: err, at: time.Now(), sysInfo: sys, wantStats: remote}
		}
		groups := docker.ComposeGroupsFrom(containers)
		if pane == 1 {
			// Right pane: skip images/volumes/networks (not shown in multi).
			return dataMsg{
				gen:        gen,
				pane:       pane,
				groups:     groups,
				containers: containers,
				sysInfo:    sys,
				at:         time.Now(),
				wantStats:  remote,
			}
		}
		images, err := client.ListImages(ctx)
		if err != nil {
			return dataMsg{gen: gen, pane: pane, err: err, at: time.Now(), sysInfo: sys, wantStats: remote}
		}
		volumes, err := client.ListVolumes(ctx)
		if err != nil {
			return dataMsg{gen: gen, pane: pane, err: err, at: time.Now(), sysInfo: sys, wantStats: remote}
		}
		networks, err := client.ListNetworks(ctx)
		if err != nil {
			return dataMsg{gen: gen, pane: pane, err: err, at: time.Now(), sysInfo: sys, wantStats: remote}
		}
		return dataMsg{
			gen:        gen,
			pane:       pane,
			groups:     groups,
			containers: containers,
			images:     images,
			volumes:    volumes,
			networks:   networks,
			sysInfo:    sys,
			at:         time.Now(),
			wantStats:  remote,
		}
	}
}

func (m Model) enrichPaneStatsCmd(pane int, gen uint64, containers []docker.ContainerInfo) tea.Cmd {
	var client *docker.Client
	if pane == 1 {
		client = m.clientRight
	} else {
		client = m.client
	}
	snapshot := append([]docker.ContainerInfo(nil), containers...)
	return func() tea.Msg {
		if client == nil {
			return statsEnrichMsg{gen: gen, pane: pane, err: fmt.Errorf("no client")}
		}
		timeout := 90 * time.Second
		if !client.IsRemoteDaemon() {
			timeout = 30 * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		filled := client.EnrichContainerStats(ctx, snapshot)
		return statsEnrichMsg{
			gen:        gen,
			pane:       pane,
			containers: filled,
			groups:     docker.ComposeGroupsFrom(filled),
			at:         time.Now(),
		}
	}
}

func (m Model) tickMultiHostCmds() []tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, m.tickOnePane(0)...)
	if m.clientRight != nil {
		cmds = append(cmds, m.tickOnePane(1)...)
	}
	return cmds
}

func (m *Model) tickOnePane(pane int) []tea.Cmd {
	var cmds []tea.Cmd
	if pane == 0 {
		if m.loading || m.busy {
			return nil
		}
		remote := m.client != nil && m.client.IsRemoteDaemon()
		if remote && len(m.containers) > 0 {
			m.remoteTickCount++
			if m.remoteTickCount%3 == 0 {
				m.dataGen++
				cmds = append(cmds, m.refreshPaneCmd(0, false, m.dataGen))
			} else if !m.statsEnrichBusy && hasRunningContainers(m.containers) {
				m.statsEnrichBusy = true
				cmds = append(cmds, m.enrichPaneStatsCmd(0, m.dataGen, m.containers))
			}
		} else {
			m.dataGen++
			cmds = append(cmds, m.refreshPaneCmd(0, true, m.dataGen))
		}
		return cmds
	}
	if m.loadingRight || m.clientRight == nil {
		return nil
	}
	remote := m.clientRight.IsRemoteDaemon()
	if remote && len(m.containersRight) > 0 {
		m.remoteTickCountRight++
		if m.remoteTickCountRight%3 == 0 {
			m.dataGenRight++
			cmds = append(cmds, m.refreshPaneCmd(1, false, m.dataGenRight))
		} else if !m.statsEnrichBusyRight && hasRunningContainers(m.containersRight) {
			m.statsEnrichBusyRight = true
			cmds = append(cmds, m.enrichPaneStatsCmd(1, m.dataGenRight, m.containersRight))
		}
	} else {
		m.dataGenRight++
		cmds = append(cmds, m.refreshPaneCmd(1, true, m.dataGenRight))
	}
	return cmds
}
