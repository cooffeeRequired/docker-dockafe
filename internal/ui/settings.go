package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type settingsItem int

const (
	settingsHosts settingsItem = iota
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
	return []string{
		fmt.Sprintf("Docker hosts          %s", truncate(host, max(12, m.width/3))),
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

	meta := metaStyle.Render("settings · Enter open · H hosts shortcut")

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
	b.WriteString(chartLabelStyle().Render("Configure connection targets, updates, and app info."))

	frame := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
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
		{"Enter", "open"},
		{"H", "hosts"},
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
		case settingsUpdate:
			return m.askUpdate()
		case settingsAbout:
			m.status = fmt.Sprintf("%s %s · host %s", AppName, AppVersion, m.dockerHostLabel())
			return m, nil
		}
		return m, nil
	case "H":
		return m.openHosts()
	case "U":
		return m.askUpdate()
	}
	return m, nil
}
