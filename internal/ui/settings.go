package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cooffeeRequired/dockafe/internal/config"
)

type settingsItem int

const (
	settingsHosts settingsItem = iota
	settingsRemoteWrite
	settingsAudit
	settingsUpdate
	settingsAbout
	settingsItemCount
)

func (m Model) settingsLabels() []string {
	host := m.dockerHostLabel()
	update := "up to date"
	if m.updateAvailable {
		update = "available · " + m.updateLatest
	}
	remoteWrite := "n/a (local host)"
	if m.client != nil && m.client.IsRemoteDaemon() {
		if m.settings.IsRemoteReadOnly() {
			remoteWrite = "locked · Enter unlock"
		} else {
			remoteWrite = "unlocked · Enter lock"
		}
	} else if m.settings.IsRemoteReadOnly() {
		remoteWrite = "locked (applies on remote)"
	} else {
		remoteWrite = "unlocked (applies on remote)"
	}
	auditPath, _ := config.AuditPath()
	if auditPath == "" {
		auditPath = "~/.config/dockafe/audit.log"
	}
	return []string{
		fmt.Sprintf("Docker hosts          %s", truncate(host, max(12, m.width/3))),
		fmt.Sprintf("Remote write          %s", remoteWrite),
		fmt.Sprintf("Audit log             %s", truncate(auditPath, max(20, m.width/2))),
		fmt.Sprintf("Check for updates     %s", update),
		fmt.Sprintf("About                 %s %s", AppName, AppVersion),
	}
}

func (m Model) viewSettings() string {
	var tabs []string
	for i, name := range tabNames {
		style := tabStyle
		if Tab(i) == m.tab {
			style = activeTabStyle
		}
		tabs = append(tabs, style.Render(fmt.Sprintf(" %d:%s ", i+1, name)))
	}
	top := lipgloss.JoinHorizontal(lipgloss.Top,
		titleStyle.Render(m.versionTitle()),
		"  ",
		lipgloss.JoinHorizontal(lipgloss.Top, tabs...),
	)
	if badge := m.updateBadge(); badge != "" {
		top = lipgloss.JoinHorizontal(lipgloss.Top, top, " ", badge)
	}
	if m.remoteReadOnlyActive() {
		top = lipgloss.JoinHorizontal(lipgloss.Top, top, " ",
			errorStyle.Render(" read-only "))
	}

	meta := metaStyle.Render("settings · Enter open/toggle · H hosts")

	var b strings.Builder
	b.WriteString(helpTitleStyle.Render("Settings"))
	b.WriteString("\n\n")
	labels := m.settingsLabels()
	for i, label := range labels {
		mark := "  "
		line := mark + label
		if settingsItem(i) == m.settingsCursor {
			mark = "> "
			line = mark + label
			b.WriteString(activeTabStyle.Render(line))
		} else {
			b.WriteString(helpDescStyle.Render(line))
		}
		b.WriteByte('\n')
	}
	b.WriteString("\n")
	b.WriteString(chartLabelStyle().Render("Remote write lock blocks start/stop/remove/prune/upload on ssh/tcp hosts."))

	frame := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cBorder).
		Width(max(20, m.width-2)).
		Padding(0, 1)
	body := frame.Render(b.String())

	statusLine := m.status
	if m.loading {
		statusLine = "loading…"
	}
	if m.errMsg != "" {
		statusLine = errorStyle.Render("ERR: " + truncate(m.errMsg, max(20, m.width-10)))
	} else if m.eventAlert != "" {
		statusLine = errorStyle.Render("EVT: " + truncate(m.eventAlert, max(20, m.width-10)))
	} else {
		statusLine = statusStyle.Render(statusLine)
	}

	sys := ""
	if m.sysInfo != "" {
		sys = metaStyle.Render(truncate(m.sysInfo, m.width-2))
	}

	help := renderHelpRow("set", []helpBinding{
		{"↑↓", "move"},
		{"Enter", "open/toggle"},
		{"H", "hosts"},
		{"M", "multi"},
		{"U", "update"},
		{"1-6", "tabs"},
		{"q", "quit"},
	})

	parts := []string{top, meta, body, statusLine}
	if sys != "" {
		parts = append(parts, sys)
	}
	parts = append(parts, help)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m Model) handleSettingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "up", "k":
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}
		return m, nil
	case "down", "j":
		if int(m.settingsCursor) < int(settingsItemCount)-1 {
			m.settingsCursor++
		}
		return m, nil
	case "enter", " ":
		switch m.settingsCursor {
		case settingsHosts:
			return m.openHosts()
		case settingsRemoteWrite:
			locked := !m.settings.IsRemoteReadOnly()
			s, err := config.SetRemoteReadOnly(locked)
			if err != nil {
				m.errMsg = err.Error()
				m.status = "settings save failed"
				return m, nil
			}
			m.settings = s
			m.errMsg = ""
			if locked {
				m.status = "remote write locked"
			} else {
				m.status = "remote write unlocked"
			}
			return m, nil
		case settingsAudit:
			return m.openAuditLog()
		case settingsUpdate:
			return m.askUpdate()
		case settingsAbout:
			ro := ""
			if m.remoteReadOnlyActive() {
				ro = " · read-only"
			}
			m.status = fmt.Sprintf("%s %s · host %s%s", AppName, AppVersion, m.dockerHostLabel(), ro)
			return m, nil
		}
		return m, nil
	case "H":
		m.hostPickTarget = hostPickLeft
		return m.openHosts()
	case "M":
		return m.openMultiHost()
	case "U":
		return m.askUpdate()
	}
	return m, nil
}

func (m Model) openAuditLog() (tea.Model, tea.Cmd) {
	body, err := config.AuditTail(80)
	if err != nil {
		m.errMsg = err.Error()
		m.status = "audit log read failed"
		return m, nil
	}
	m.mode = ModeDetail
	m.detailTitle = "Audit log"
	m.detailBody = body
	m.vp.SetContent(body)
	m.vp.GotoBottom()
	m.relayout()
	m.status = "audit log · esc back"
	return m, nil
}
