package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cooffeeRequired/dockafe/internal/docker"
)

type hostSwitchMsg struct {
	client *docker.Client
	err    error
	label  string
}

type hostsLoadedMsg struct {
	endpoints []docker.Endpoint
	err       error
}

func (m Model) openHosts() (tea.Model, tea.Cmd) {
	if m.client != nil && m.client.IsDemo() {
		m.status = "host switch disabled in demo mode"
		return m, nil
	}
	m.mode = ModeHosts
	m.hostCursor = 0
	m.hostCustom = false
	m.hostErr = ""
	m.status = "select Docker host · Enter connect · c custom URL · esc back"
	ti := textinput.New()
	ti.Placeholder = "ssh://user@host | tcp://host:2375 | unix:///var/run/docker.sock"
	ti.CharLimit = 256
	ti.Width = max(40, m.width-8)
	m.hostInput = ti

	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		eps, err := docker.ListEndpoints(ctx)
		return hostsLoadedMsg{endpoints: eps, err: err}
	}
}

func (m Model) viewHosts() string {
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		titleStyle.Render(m.versionTitle()),
		"  ",
		activeTabStyle.Render(" HOSTS "),
		"  ",
		metaStyle.Render("current: "+m.dockerHostLabel()),
	)

	var b strings.Builder
	b.WriteString(helpTitleStyle.Render("Docker hosts / contexts"))
	b.WriteString("\n\n")
	if m.hostErr != "" {
		b.WriteString(errorStyle.Render(m.hostErr))
		b.WriteString("\n\n")
	}
	if m.hostCustom {
		b.WriteString(filterStyle.Render("URL: " + m.hostInput.View()))
		b.WriteString("\n\n")
		b.WriteString(helpDescStyle.Render("Enter connect · esc cancel custom"))
	} else {
		if len(m.hostEndpoints) == 0 {
			b.WriteString(chartLabelStyle().Render("No contexts found — press c to enter a host URL."))
		}
		for i, ep := range m.hostEndpoints {
			mark := "  "
			if i == m.hostCursor {
				mark = "> "
			}
			cur := ""
			if ep.Current {
				cur = " *"
			}
			line := fmt.Sprintf("%s%s  %s%s  [%s]", mark, ep.Name, ep.Host, cur, ep.Source)
			if i == m.hostCursor {
				b.WriteString(activeTabStyle.Render(line))
			} else {
				b.WriteString(helpDescStyle.Render(line))
			}
			b.WriteByte('\n')
		}
		b.WriteString("\n")
		b.WriteString(helpDescStyle.Render("c custom · Enter connect · esc back"))
	}

	frame := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(max(20, m.width-2)).
		Padding(0, 1)
	body := frame.Render(b.String())
	status := statusStyle.Render(m.status)
	if m.errMsg != "" {
		status = errorStyle.Render("ERR: " + truncate(m.errMsg, m.width-10))
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, status)
}

func (m Model) dockerHostLabel() string {
	if m.client == nil {
		return "-"
	}
	h := m.client.Host()
	if h == "" {
		return "-"
	}
	return h
}

func (m Model) handleHostsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.hostCustom {
		switch key {
		case "esc":
			m.hostCustom = false
			m.hostInput.Blur()
			m.status = "select Docker host"
			return m, nil
		case "enter":
			host := strings.TrimSpace(m.hostInput.Value())
			if host == "" {
				m.hostErr = "empty host URL"
				return m, nil
			}
			m.busy = true
			m.status = "connecting…"
			return m, m.connectHostCmd(host)
		case "ctrl+c":
			return m, tea.Quit
		}
		var cmd tea.Cmd
		m.hostInput, cmd = m.hostInput.Update(msg)
		return m, cmd
	}

	switch key {
	case "esc", "q", "H":
		m.mode = ModeList
		if m.tab != TabSettings {
			m.tab = TabSettings
		}
		m.relayout()
		m.status = "settings"
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "c":
		m.hostCustom = true
		m.hostInput.SetValue("")
		m.hostInput.Focus()
		m.status = "enter Docker host URL"
		return m, nil
	case "up", "k":
		if m.hostCursor > 0 {
			m.hostCursor--
		}
		return m, nil
	case "down", "j":
		if m.hostCursor < len(m.hostEndpoints)-1 {
			m.hostCursor++
		}
		return m, nil
	case "enter":
		if m.hostCursor < 0 || m.hostCursor >= len(m.hostEndpoints) {
			return m, nil
		}
		ep := m.hostEndpoints[m.hostCursor]
		m.busy = true
		m.status = "connecting…"
		return m, m.connectHostCmd(ep.Host)
	}
	return m, nil
}

func (m Model) connectHostCmd(host string) tea.Cmd {
	return func() tea.Msg {
		cli, err := docker.NewWithHost(host)
		if err != nil {
			return hostSwitchMsg{err: err, label: host}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := cli.Ping(ctx); err != nil {
			_ = cli.Close()
			return hostSwitchMsg{err: fmt.Errorf("ping %s: %w", host, err), label: host}
		}
		return hostSwitchMsg{client: cli, label: cli.Host()}
	}
}

func (m *Model) applyHostSwitch(msg hostSwitchMsg) tea.Cmd {
	if msg.err != nil {
		m.hostErr = msg.err.Error()
		m.errMsg = msg.err.Error()
		m.status = "connect failed"
		m.busy = false
		return nil
	}
	m.stopEventWatch()
	old := m.client
	m.client = msg.client
	if old != nil && !old.IsDemo() {
		_ = old.Close()
	}
	m.hostErr = ""
	m.errMsg = ""
	m.mode = ModeList
	m.tab = TabSettings
	m.settingsCursor = settingsHosts
	m.statsHist = map[string]*statsSeries{}
	m.eventLog = nil
	m.eventAlert = ""
	m.volListCache = map[string][]docker.VolumeEntry{}
	m.status = "connected · " + msg.label
	m.loading = true
	m.busy = false
	m.relayout()
	return tea.Batch(m.startRefresh(true), startEventWatchCmd(m.client))
}

type volTransferKind int

const (
	volTransferNone volTransferKind = iota
	volTransferDownload
	volTransferUpload
	volTransferMoveDown
	volTransferMoveUp
)

type volTransferDoneMsg struct {
	kind volTransferKind
	path string
	err  error
}

func (m Model) beginVolTransfer(kind volTransferKind) (tea.Model, tea.Cmd) {
	n := m.selectedVolNode()
	if n == nil {
		m.status = "select a file or directory"
		return m, nil
	}
	ti := textinput.New()
	ti.CharLimit = 512
	ti.Width = max(40, m.width-8)
	ti.Focus()

	switch kind {
	case volTransferDownload, volTransferMoveDown:
		ti.Placeholder = "local destination path"
		ti.SetValue(docker.SuggestLocalExportPath(m.volName, n.entry.Path, n.entry.IsDir))
		m.volTransferPath = n.entry.Path
		m.volTransferIsDir = n.entry.IsDir
		m.status = "export volume → local · Enter confirm · esc cancel"
	case volTransferUpload, volTransferMoveUp:
		ti.Placeholder = "local source file path"
		ti.SetValue("")
		dest := n.entry.Path
		if n.entry.IsDir {
			dest = filepath.ToSlash(filepath.Join(n.entry.Path, "uploaded-file"))
		}
		m.volTransferPath = dest
		m.volTransferIsDir = false
		m.status = "import local → volume " + dest + " · Enter · esc"
	default:
		return m, nil
	}

	m.volTransferKind = kind
	m.volTransferInput = ti
	m.mode = ModeVolTransfer
	return m, nil
}

func (m Model) viewVolTransfer() string {
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		titleStyle.Render(m.versionTitle()),
		"  ",
		activeTabStyle.Render(" TRANSFER "),
		"  ",
		metaStyle.Render(m.volName+" · "+m.volTransferPath),
	)
	action := "copy"
	dir := "volume → local"
	switch m.volTransferKind {
	case volTransferMoveDown:
		action = "move"
	case volTransferUpload:
		dir = "local → volume"
	case volTransferMoveUp:
		action = "move"
		dir = "local → volume"
	}
	body := helpTitleStyle.Render(action+" · "+dir) + "\n\n" +
		filterStyle.Render(m.volTransferInput.View()) + "\n\n" +
		helpDescStyle.Render("Enter run · esc cancel")
	frame := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(max(20, m.width-2)).
		Padding(0, 1)
	return lipgloss.JoinVertical(lipgloss.Left, header, frame.Render(body), statusStyle.Render(m.status))
}

func (m Model) handleVolTransferKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.volTransferKind = volTransferNone
		m.mode = ModeVolumeTree
		m.status = "transfer cancelled"
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "enter":
		localPath := strings.TrimSpace(m.volTransferInput.Value())
		if localPath == "" {
			m.status = "path required"
			return m, nil
		}
		kind := m.volTransferKind
		vol := m.volName
		rel := m.volTransferPath
		isDir := m.volTransferIsDir
		client := m.client
		m.busy = true
		m.status = "transferring…"
		m.mode = ModeVolumeTree
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			var err error
			switch kind {
			case volTransferDownload:
				err = client.ExportVolumePathToLocal(ctx, vol, rel, localPath, isDir)
			case volTransferMoveDown:
				err = client.ExportVolumePathToLocal(ctx, vol, rel, localPath, isDir)
				if err == nil {
					err = client.RemoveVolumePath(ctx, vol, rel)
				}
			case volTransferUpload:
				dest := rel
				if dest == "" {
					dest = filepath.Base(localPath)
				}
				err = client.ImportLocalFileToVolume(ctx, vol, dest, localPath)
			case volTransferMoveUp:
				dest := rel
				if dest == "" {
					dest = filepath.Base(localPath)
				}
				err = client.ImportLocalFileToVolume(ctx, vol, dest, localPath)
				if err == nil {
					err = os.Remove(localPath)
				}
			}
			return volTransferDoneMsg{kind: kind, path: localPath, err: err}
		}
	}
	var cmd tea.Cmd
	m.volTransferInput, cmd = m.volTransferInput.Update(msg)
	return m, cmd
}
