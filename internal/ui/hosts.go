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

	"github.com/cooffeeRequired/dockafe/internal/config"
	"github.com/cooffeeRequired/dockafe/internal/docker"
)

type hostFormKind int

const (
	hostFormNone hostFormKind = iota
	hostFormCustom
	hostFormAddName
	hostFormAddURL
	hostFormSaveName
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

type hostSavedMsg struct {
	err error
	msg string
}

func (m Model) openHosts() (tea.Model, tea.Cmd) {
	if m.client != nil && m.client.IsDemo() {
		m.status = "host switch disabled in demo mode"
		return m, nil
	}
	m.mode = ModeHosts
	m.hostCursor = 0
	m.hostForm = hostFormNone
	m.hostDraftName = ""
	m.hostErr = ""
	target := "LEFT"
	if m.hostPickTarget == hostPickRight {
		target = "RIGHT"
	}
	m.status = "select Docker host for " + target + " · a add · Enter connect · esc back"
	ti := textinput.New()
	ti.Placeholder = "ssh://user@host | tcp://host:2375 | unix:///var/run/docker.sock"
	ti.CharLimit = 256
	ti.Width = max(40, m.width-8)
	m.hostInput = ti

	return m, m.loadHostsCmd()
}

func (m Model) loadHostsCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		eps, err := docker.ListEndpoints(ctx)
		if err != nil {
			return hostsLoadedMsg{err: err}
		}
		return hostsLoadedMsg{endpoints: mergeSavedEndpoints(eps), err: nil}
	}
}

func mergeSavedEndpoints(eps []docker.Endpoint) []docker.Endpoint {
	saved, err := config.LoadHosts()
	if err != nil || len(saved) == 0 {
		return eps
	}
	seen := map[string]bool{}
	out := make([]docker.Endpoint, 0, len(saved)+len(eps))
	for _, h := range saved {
		key := strings.ToLower(h.Host)
		seen[key] = true
		name := h.Name
		if h.Note != "" {
			name = h.Name + " — " + h.Note
		}
		out = append(out, docker.Endpoint{
			Name:   name,
			Host:   h.Host,
			Source: "saved",
		})
	}
	for _, ep := range eps {
		if seen[strings.ToLower(ep.Host)] {
			continue
		}
		out = append(out, ep)
	}
	return out
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
	b.WriteString(helpTitleStyle.Render("Docker hosts / favorites"))
	b.WriteString("\n\n")
	if m.hostErr != "" {
		b.WriteString(errorStyle.Render(m.hostErr))
		b.WriteString("\n\n")
	}

	switch m.hostForm {
	case hostFormCustom:
		b.WriteString(filterStyle.Render("URL: " + m.hostInput.View()))
		b.WriteString("\n\n")
		b.WriteString(helpDescStyle.Render("Enter connect · esc cancel"))
	case hostFormAddName:
		b.WriteString(helpDescStyle.Render("New favorite — name (e.g. produkce)"))
		b.WriteString("\n")
		b.WriteString(filterStyle.Render("Name: " + m.hostInput.View()))
		b.WriteString("\n\n")
		b.WriteString(helpDescStyle.Render("Enter next · esc cancel"))
	case hostFormAddURL:
		b.WriteString(helpDescStyle.Render("Favorite · " + m.hostDraftName))
		b.WriteString("\n")
		b.WriteString(filterStyle.Render("URL: " + m.hostInput.View()))
		b.WriteString("\n\n")
		b.WriteString(helpDescStyle.Render("Enter save · esc cancel"))
	case hostFormSaveName:
		b.WriteString(helpDescStyle.Render("Save current host as favorite"))
		b.WriteString("\n")
		b.WriteString(chartLabelStyle().Render(m.dockerHostLabel()))
		b.WriteString("\n")
		b.WriteString(filterStyle.Render("Name: " + m.hostInput.View()))
		b.WriteString("\n\n")
		b.WriteString(helpDescStyle.Render("Enter save · esc cancel"))
	default:
		if len(m.hostEndpoints) == 0 {
			b.WriteString(chartLabelStyle().Render("No hosts yet — press a to add (e.g. production ssh://…)."))
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
		b.WriteString(helpDescStyle.Render("a add · s save current · d delete saved · c custom · Enter connect · esc back"))
	}

	frame := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cBorder).
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

func (m Model) beginHostInput(form hostFormKind, placeholder, value, status string) (tea.Model, tea.Cmd) {
	m.hostForm = form
	m.hostErr = ""
	m.hostInput.Placeholder = placeholder
	m.hostInput.SetValue(value)
	m.hostInput.CursorEnd()
	m.hostInput.Focus()
	m.status = status
	return m, nil
}

func (m Model) cancelHostForm() (tea.Model, tea.Cmd) {
	m.hostForm = hostFormNone
	m.hostDraftName = ""
	m.hostInput.Blur()
	m.hostInput.SetValue("")
	m.status = "select Docker host"
	return m, nil
}

func (m Model) handleHostsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.hostForm != hostFormNone {
		switch key {
		case "esc":
			return m.cancelHostForm()
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			return m.submitHostForm()
		}
		var cmd tea.Cmd
		m.hostInput, cmd = m.hostInput.Update(msg)
		return m, cmd
	}

	switch key {
	case "esc", "q", "H":
		if m.hostPickTarget == hostPickRight {
			m.hostPickTarget = hostPickLeft
			if m.clientRight != nil {
				m.mode = ModeMultiHost
				m.status = "multi-host"
			} else {
				m.mode = ModeList
				m.status = "multi-host cancelled"
			}
			m.relayout()
			return m, nil
		}
		if m.clientRight != nil && m.returnToMulti {
			m.mode = ModeMultiHost
			m.relayout()
			m.status = "multi-host"
			return m, nil
		}
		m.mode = ModeList
		if m.tab != TabSettings {
			m.tab = TabSettings
		}
		m.relayout()
		m.status = "settings"
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "a":
		m.hostDraftName = ""
		return m.beginHostInput(hostFormAddName, "produkce", "", "name for saved host")
	case "s":
		cur := m.dockerHostLabel()
		if cur == "" || cur == "-" || cur == "demo" {
			m.hostErr = "nothing to save — connect first"
			return m, nil
		}
		suggest := suggestHostName(cur)
		return m.beginHostInput(hostFormSaveName, suggest, suggest, "name for current host")
	case "d":
		if m.hostCursor < 0 || m.hostCursor >= len(m.hostEndpoints) {
			return m, nil
		}
		ep := m.hostEndpoints[m.hostCursor]
		if ep.Source != "saved" {
			m.hostErr = "only saved favorites can be deleted (source=saved)"
			m.status = "delete skipped"
			return m, nil
		}
		return m, func() tea.Msg {
			err := config.RemoveHost(ep.Host)
			if err != nil {
				return hostSavedMsg{err: err}
			}
			return hostSavedMsg{msg: "removed " + ep.Name}
		}
	case "c":
		return m.beginHostInput(
			hostFormCustom,
			"ssh://user@host | tcp://host:2375 | unix:///var/run/docker.sock",
			"",
			"enter Docker host URL",
		)
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

func (m Model) submitHostForm() (tea.Model, tea.Cmd) {
	val := strings.TrimSpace(m.hostInput.Value())
	switch m.hostForm {
	case hostFormCustom:
		if val == "" {
			m.hostErr = "empty host URL"
			return m, nil
		}
		m.busy = true
		m.status = "connecting…"
		return m, m.connectHostCmd(val)
	case hostFormAddName:
		if val == "" {
			m.hostErr = "name required"
			return m, nil
		}
		m.hostDraftName = val
		return m.beginHostInput(
			hostFormAddURL,
			"ssh://user@host",
			"",
			"URL for "+val,
		)
	case hostFormAddURL:
		if val == "" {
			m.hostErr = "host URL required"
			return m, nil
		}
		name := m.hostDraftName
		return m, func() tea.Msg {
			err := config.UpsertHost(name, val, "")
			if err != nil {
				return hostSavedMsg{err: err}
			}
			return hostSavedMsg{msg: "saved " + name}
		}
	case hostFormSaveName:
		if val == "" {
			m.hostErr = "name required"
			return m, nil
		}
		host := m.dockerHostLabel()
		return m, func() tea.Msg {
			err := config.UpsertHost(val, host, "")
			if err != nil {
				return hostSavedMsg{err: err}
			}
			return hostSavedMsg{msg: "saved " + val}
		}
	}
	return m, nil
}

func suggestHostName(host string) string {
	host = strings.TrimSpace(host)
	host = strings.TrimPrefix(host, "ssh://")
	host = strings.TrimPrefix(host, "tcp://")
	host = strings.TrimPrefix(host, "unix://")
	if i := strings.Index(host, "@"); i >= 0 {
		host = host[i+1:]
	}
	if i := strings.IndexAny(host, ":/"); i >= 0 {
		host = host[:i]
	}
	host = strings.TrimSpace(host)
	if host == "" || host == "var" {
		return "remote"
	}
	return host
}

func (m Model) connectHostCmd(host string) tea.Cmd {
	return func() tea.Msg {
		cli, err := docker.NewWithHost(host)
		if err != nil {
			return hostSwitchMsg{err: err, label: host}
		}
		// SSH dials via OpenSSH (ConnectTimeout=30); allow headroom for auth + docker dial-stdio.
		timeout := 8 * time.Second
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(host)), "ssh://") {
			timeout = 45 * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
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
	if m.hostPickTarget == hostPickRight {
		return m.applyRightHost(msg)
	}

	m.stopEventWatch()
	old := m.client
	m.client = msg.client
	if old != nil && !old.IsDemo() {
		_ = old.Close()
	}
	m.hostErr = ""
	m.errMsg = ""
	m.hostForm = hostFormNone
	m.hostDraftName = ""
	m.hostInput.Blur()
	m.statsHist = map[string]*statsSeries{}
	m.eventLog = nil
	m.eventAlert = ""
	m.volListCache = map[string][]docker.VolumeEntry{}
	m.statsEnrichBusy = false
	m.remoteTickCount = 0
	m.sysInfo = ""
	m.groups = nil
	m.containers = nil
	m.images = nil
	m.volumes = nil
	m.networks = nil
	m.status = "connected · " + msg.label + " · syncing…"
	m.loading = true
	m.busy = false
	m.actionPane = 0
	if m.clientRight != nil {
		m.mode = ModeMultiHost
		m.multiFocus = 0
		m.returnToMulti = true
	} else {
		m.mode = ModeList
		m.tab = TabSettings
		m.settingsCursor = settingsHosts
	}
	m.relayout()
	return tea.Batch(m.startRefresh(false), startEventWatchCmd(m.client))
}

func (m *Model) applyRightHost(msg hostSwitchMsg) tea.Cmd {
	old := m.clientRight
	m.clientRight = msg.client
	if old != nil {
		_ = old.Close()
	}
	m.hostPickTarget = hostPickLeft
	m.hostErr = ""
	m.errMsg = ""
	m.hostForm = hostFormNone
	m.hostDraftName = ""
	m.hostInput.Blur()
	m.mode = ModeMultiHost
	m.multiFocus = 1
	m.returnToMulti = true
	m.loadingRight = true
	m.statsEnrichBusyRight = false
	m.remoteTickCountRight = 0
	m.groupsRight = nil
	m.containersRight = nil
	m.sysInfoRight = ""
	m.errRight = ""
	m.dataGenRight++
	m.busy = false
	m.status = "right · " + msg.label + " · syncing…"
	m.relayout()
	return m.refreshPaneCmd(1, false, m.dataGenRight)
}

func (m *Model) applyHostSaved(msg hostSavedMsg) tea.Cmd {
	m.busy = false
	if msg.err != nil {
		m.hostErr = msg.err.Error()
		m.status = "save failed"
		return nil
	}
	m.hostErr = ""
	m.hostForm = hostFormNone
	m.hostDraftName = ""
	m.hostInput.Blur()
	m.hostInput.SetValue("")
	m.status = msg.msg
	return m.loadHostsCmd()
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
	// Download (copy out) is allowed on remote read-only; upload/move mutate the daemon.
	if kind != volTransferDownload {
		if m2, ok := m.guardMutate(); !ok {
			return m2, nil
		}
	}
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
		BorderForeground(cBorder).
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
		host := m.auditHost()
		m.busy = true
		m.status = "transferring…"
		m.mode = ModeVolumeTree
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			var err error
			action := "volume_download"
			switch kind {
			case volTransferDownload:
				err = client.ExportVolumePathToLocal(ctx, vol, rel, localPath, isDir)
			case volTransferMoveDown:
				action = "volume_move_down"
				err = client.ExportVolumePathToLocal(ctx, vol, rel, localPath, isDir)
				if err == nil {
					err = client.RemoveVolumePath(ctx, vol, rel)
				}
			case volTransferUpload:
				action = "volume_upload"
				dest := rel
				if dest == "" {
					dest = filepath.Base(localPath)
				}
				err = client.ImportLocalFileToVolume(ctx, vol, dest, localPath)
			case volTransferMoveUp:
				action = "volume_move_up"
				dest := rel
				if dest == "" {
					dest = filepath.Base(localPath)
				}
				err = client.ImportLocalFileToVolume(ctx, vol, dest, localPath)
				if err == nil {
					err = os.Remove(localPath)
				}
			}
			errMsg := ""
			if err != nil {
				errMsg = err.Error()
			}
			_ = config.Audit(host, action, vol+"/"+rel, err == nil, errMsg)
			return volTransferDoneMsg{kind: kind, path: localPath, err: err}
		}
	}
	var cmd tea.Cmd
	m.volTransferInput, cmd = m.volTransferInput.Update(msg)
	return m, cmd
}
