package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) openGraphs() (tea.Model, tea.Cmd) {
	switch m.tab {
	case TabCompose:
		name := m.currentComposeName()
		if name == "" {
			m.status = "select a compose project for graphs"
			return m, nil
		}
		m.graphsKey = composeHistKey(name)
		m.graphsTitle = "Compose · " + name
		m.detailTitle = "Graphs · " + name
	case TabContainers:
		id, name := m.currentContainer()
		if id == "" {
			m.status = "select a container for graphs"
			return m, nil
		}
		m.graphsKey = id
		m.graphsTitle = name
		m.detailTitle = "Graphs · " + name
	default:
		m.status = "graphs only on Compose/Containers"
		return m, nil
	}

	m.mode = ModeGraphs
	m.errMsg = ""
	m.status = "live CPU/MEM · esc back · refreshes with list"
	m.relayout()
	m.vp.SetContent(m.renderGraphsBody())
	m.vp.GotoTop()
	return m, nil
}

func (m *Model) refreshGraphsView() {
	if m.mode != ModeGraphs {
		return
	}
	m.vp.SetContent(m.renderGraphsBody())
}

func (m Model) renderGraphsBody() string {
	series := m.statsHist[m.graphsKey]
	width := max(20, m.vp.Width-2)
	chartH := max(4, min(10, m.vp.Height/2-3))

	var b strings.Builder
	b.WriteString(helpTitleStyle.Render(m.graphsTitle))
	b.WriteString("\n")
	b.WriteString(chartLabelStyle().Render("Samples every ~4s · window up to 60"))
	b.WriteString("\n\n")

	if series == nil || series.len() == 0 {
		b.WriteString(chartLabelStyle().Render("No samples yet — wait for the next refresh (running targets only)."))
		b.WriteByte('\n')
		return b.String()
	}

	last, _ := series.last()
	cpuVals := series.cpuValues()
	memVals := series.memValues()
	cpuMin, cpuMax := minMax(cpuVals)
	memMin, memMax := minMax(memVals)

	b.WriteString(chartValueStyle().Render(fmt.Sprintf("CPU  %.1f%%", last.cpu)))
	b.WriteString(chartLabelStyle().Render(fmt.Sprintf("   min %.1f · max %.1f · n=%d", cpuMin, cpuMax, len(cpuVals))))
	b.WriteByte('\n')
	b.WriteString(chartBlockStyle().Render(sparkline(cpuVals, width)))
	b.WriteByte('\n')
	b.WriteString(chartBlockStyle().Render(blockChart(cpuVals, width, chartH)))
	b.WriteString("\n\n")

	b.WriteString(chartValueStyle().Render(fmt.Sprintf("MEM  %s", formatMemBytes(last.mem))))
	b.WriteString(chartLabelStyle().Render(fmt.Sprintf("   min %s · max %s", formatMemBytes(uint64(memMin)), formatMemBytes(uint64(memMax)))))
	b.WriteByte('\n')
	b.WriteString(chartBlockStyle().Render(sparkline(memVals, width)))
	b.WriteByte('\n')
	b.WriteString(chartBlockStyle().Render(blockChart(memVals, width, chartH)))
	b.WriteByte('\n')

	return b.String()
}

func (m Model) viewGraphs() string {
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		titleStyle.Render(m.versionTitle()),
		"  ",
		activeTabStyle.Render(" GRAPHS "),
		"  ",
		metaStyle.Render(m.detailTitle),
	)
	if badge := m.updateBadge(); badge != "" {
		header = lipgloss.JoinHorizontal(lipgloss.Top, header, " ", badge)
	}

	frame := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(max(20, m.width-2))

	body := frame.Render(m.vp.View())
	help := renderHelpRow("graphs", []helpBinding{
		{"esc", "back"},
		{"r", "refresh"},
		{"q", "quit"},
	})
	status := statusStyle.Render(m.status)
	if m.errMsg != "" {
		status = errorStyle.Render("ERR: " + truncate(m.errMsg, m.width-10))
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, status, help)
}
