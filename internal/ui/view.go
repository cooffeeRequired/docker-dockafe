package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) relayout() {
	if m.width < 40 || m.height < 10 {
		return
	}
	header := 3
	footer := 5 // status + 2-line help + breathing room
	switch m.mode {
	case ModeDetail, ModeLogs, ModeHelp:
		m.vp.Width = max(20, m.width-4)
		m.vp.Height = max(5, m.height-header-footer-1)
	case ModeComposeDetail:
		// rendered manually
	default:
		tableH := m.height - header - footer - 1
		if tableH < 5 {
			tableH = 5
		}
		m.table.SetHeight(tableH)
		m.applyRows()
	}
}

func (m Model) View() string {
	if !m.ready {
		return "initializing…"
	}

	switch m.mode {
	case ModeSplash:
		return m.viewSplash()
	case ModeVolumeTree:
		return m.viewVolumeTree()
	case ModeCreateCompose:
		return m.viewComposeWizard()
	case ModePullImage:
		return m.viewImageWizard()
	case ModeComposeDetail:
		return m.viewComposeDetail()
	case ModeConfirm:
		if m.confirm == confirmVolWrite {
			return m.viewVolumeTree()
		}
		if m.composeProject != "" {
			return m.viewComposeDetail()
		}
		return m.viewList()
	case ModeDetail, ModeLogs, ModeHelp:
		return m.viewPanel()
	default:
		return m.viewList()
	}
}

func (m Model) viewList() string {
	var tabs []string
	for i, name := range tabNames {
		style := tabStyle
		if Tab(i) == m.tab {
			style = activeTabStyle
		}
		label := fmt.Sprintf(" %d:%s ", i+1, name)
		if Tab(i) == TabContainers && m.selectedGroup != "" {
			label = fmt.Sprintf(" %d:%s[%s] ", i+1, name, truncate(m.selectedGroup, 16))
		}
		tabs = append(tabs, style.Render(label))
	}

	top := lipgloss.JoinHorizontal(lipgloss.Top, titleStyle.Render(" "+AppName+" "+AppVersion+" "), "  ", lipgloss.JoinHorizontal(lipgloss.Top, tabs...))

	metaBits := []string{
		fmt.Sprintf("sort:%s", m.sortLabel()),
	}
	if m.filter.Value() != "" || m.mode == ModeFilter {
		metaBits = append(metaBits, "filter:"+m.filter.Value())
	}
	if m.runningOnly {
		if m.tab == TabVolumes {
			metaBits = append(metaBits, "in-use-only")
		} else {
			metaBits = append(metaBits, "running-only")
		}
	}
	if m.selectedGroup != "" {
		metaBits = append(metaBits, "project:"+m.selectedGroup)
	}
	meta := metaStyle.Render(strings.Join(metaBits, " · "))

	var filterLine string
	if m.mode == ModeFilter {
		filterLine = filterStyle.Render("/ " + m.filter.View())
	}

	body := m.table.View()

	statusLine := m.status
	if m.loading {
		statusLine = "loading…"
	}
	if m.busy {
		statusLine = "… " + statusLine
	}
	if m.errMsg != "" {
		statusLine = errorStyle.Render("ERR: " + truncate(m.errMsg, max(20, m.width-10)))
	} else {
		statusLine = statusStyle.Render(statusLine)
	}

	sys := ""
	if m.sysInfo != "" {
		sys = metaStyle.Render(truncate(m.sysInfo, m.width-2))
	}

	help := helpFooterList(m.tab)
	if m.mode == ModeConfirm {
		help = confirmStyle.Render(m.confirmLabel + "  [y/n]")
	}

	parts := []string{top, meta}
	if filterLine != "" {
		parts = append(parts, filterLine)
	}
	parts = append(parts, body, statusLine)
	if sys != "" {
		parts = append(parts, sys)
	}
	parts = append(parts, help)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m Model) viewPanel() string {
	modeLabel := "DETAIL"
	switch m.mode {
	case ModeLogs:
		modeLabel = "LOGS"
		if m.logsFollow {
			modeLabel += " · follow"
		}
	case ModeHelp:
		modeLabel = "HELP"
	}

	header := lipgloss.JoinHorizontal(lipgloss.Top,
		titleStyle.Render(" "+AppName+" "+AppVersion+" "),
		"  ",
		activeTabStyle.Render(" "+modeLabel+" "),
		"  ",
		metaStyle.Render(m.detailTitle),
	)

	// Don't run viewport content through Width/Height lipgloss.Render —
	// that can strip / break ANSI colors from logs.
	frame := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(max(20, m.width-2))

	body := frame.Render(m.vp.View())
	parts := []string{header, body}
	if m.mode == ModeLogs {
		if bar := m.logSearchBarView(); bar != "" {
			parts = append(parts, bar)
		}
	}
	help := helpFooterPanel(m)
	status := statusStyle.Render(m.status)
	if m.errMsg != "" {
		status = errorStyle.Render("ERR: " + truncate(m.errMsg, m.width-10))
	}
	parts = append(parts, status, help)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}
