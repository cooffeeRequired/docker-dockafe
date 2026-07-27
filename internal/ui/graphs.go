package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	m.graphsSampleBusy = true
	m.ensureGraphsDash()
	m.status = fmt.Sprintf("dashboard · conf: %d · poll: %s · esc back", statsHistCap, graphsSampleInterval)
	m.relayout()
	m.vp.SetContent(m.renderGraphsBody())
	m.vp.GotoTop()
	return m, tea.Batch(m.sampleGraphsCmd(), graphsTickCmd())
}

func (m *Model) ensureGraphsDash() {
	if m.graphsHostCPU == nil {
		m.graphsHostCPU = newMetricSeries(statsHistCap)
	}
	if m.graphsHostMem == nil {
		m.graphsHostMem = newMetricSeries(statsHistCap)
	}
	if m.graphsDisk == nil {
		m.graphsDisk = newMetricSeries(statsHistCap)
	}
}

func (m Model) sampleGraphsCmd() tea.Cmd {
	key := m.graphsKey
	client := m.focusedClient()
	ids := m.graphsTargetIDs()
	return func() tea.Msg {
		if key == "" || client == nil {
			return graphsSampleMsg{key: key, err: fmt.Errorf("no target")}
		}
		timeout := 4 * time.Second
		if client.IsRemoteDaemon() {
			timeout = 14 * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		msg := graphsSampleMsg{key: key, at: time.Now(), dockerN: len(ids)}

		if strings.HasPrefix(key, "compose:") {
			if len(ids) == 0 {
				msg.targetErr = fmt.Errorf("no running containers")
			} else {
				cpu, mem, err := client.StatsAggregate(ctx, ids)
				msg.cpu, msg.mem, msg.targetErr = cpu, mem, err
			}
		} else {
			cpu, mem, err := client.StatsOneShot(ctx, key)
			msg.cpu, msg.mem, msg.targetErr = cpu, mem, err
			if msg.dockerN == 0 {
				msg.dockerN = 1
			}
		}

		host, hostErr := client.SampleHostMetrics(ctx)
		msg.hostErr = hostErr
		if hostErr == nil {
			msg.hostSrc = host.Source
			if host.HostCPUPct >= 0 {
				msg.hostCPU = host.HostCPUPct
				msg.hostCPUOK = true
			}
			if host.HostMemPct >= 0 {
				msg.hostMem = host.HostMemPct
				msg.hostMemOK = true
			}
			if host.DiskUsedPct >= 0 {
				msg.disk = host.DiskUsedPct
				msg.diskOK = true
			}
		}

		if msg.targetErr != nil && !msg.hostCPUOK && !msg.hostMemOK {
			msg.err = msg.targetErr
		}
		return msg
	}
}

func (m Model) graphsTargetIDs() []string {
	if strings.HasPrefix(m.graphsKey, "compose:") {
		project := strings.TrimPrefix(m.graphsKey, "compose:")
		ids := make([]string, 0)
		for _, c := range m.actionContainers() {
			if !c.Running {
				continue
			}
			if c.Project == project {
				ids = append(ids, c.ID)
			}
		}
		return ids
	}
	if m.graphsKey == "" {
		return nil
	}
	return []string{m.graphsKey}
}

func (m *Model) applyGraphsSample(msg graphsSampleMsg) {
	m.ensureGraphsDash()
	if msg.targetErr == nil {
		m.seriesFor(msg.key).push(msg.cpu, msg.mem)
		m.graphsDockerN = msg.dockerN
	}
	if msg.hostCPUOK {
		m.graphsHostCPU.push(msg.hostCPU)
	}
	if msg.hostMemOK {
		m.graphsHostMem.push(msg.hostMem)
	}
	if msg.diskOK {
		m.graphsDisk.push(msg.disk)
	}
	if msg.hostSrc != "" {
		m.graphsHostSrc = msg.hostSrc
	}
}

func (m *Model) refreshGraphsView() {
	if m.mode != ModeGraphs {
		return
	}
	m.vp.SetContent(m.renderGraphsBody())
}

func (m Model) renderGraphsBody() string {
	width := max(40, m.vp.Width-2)
	gap := 2
	panelW := (width - gap) / 2
	if panelW < 28 {
		panelW = max(24, width)
	}
	chartH := max(5, min(10, (m.vp.Height-14)/3))
	if chartH < 5 {
		chartH = 5
	}

	series := m.statsHist[m.graphsKey]
	var cpuVals, memVals []float64
	if series != nil {
		cpuVals = series.cpuValues()
		memVals = series.memValues()
	}

	n := statsHistCap
	if series != nil && series.len() > 0 {
		n = series.len()
	} else if m.graphsHostCPU.len() > 0 {
		n = m.graphsHostCPU.len()
	}

	dockerN := m.graphsDockerN
	if dockerN < 1 {
		dockerN = len(m.graphsTargetIDs())
	}
	ctrLabel := "container"
	if dockerN != 1 {
		ctrLabel = "containers"
	}

	hostCPU := renderDashPanel(dashPanel{
		title:    "HOST CPU",
		subtitle: fmt.Sprintf("load %% · %d pts", n),
		values:   m.graphsHostCPU.snapshot(),
		style:    lipgloss.NewStyle().Foreground(cHostCPU),
		formatY:  formatPctShort,
		formatV:  formatPctShort,
		fromZero: true,
		width:    panelW,
		height:   chartH,
	})
	hostMem := renderDashPanel(dashPanel{
		title:    "HOST RAM",
		subtitle: fmt.Sprintf("used %% · %d pts", n),
		values:   m.graphsHostMem.snapshot(),
		style:    lipgloss.NewStyle().Foreground(cHostMem),
		formatY:  formatPctShort,
		formatV:  formatPctShort,
		fromZero: true,
		width:    panelW,
		height:   chartH,
	})
	dockCPU := renderDashPanel(dashPanel{
		title:    "DOCKER CPU",
		subtitle: fmt.Sprintf("%d %s %% · %d pts", dockerN, ctrLabel, n),
		values:   cpuVals,
		style:    lipgloss.NewStyle().Foreground(cDockCPU),
		formatY:  formatPctShort,
		formatV:  formatPctShort,
		fromZero: true,
		width:    panelW,
		height:   chartH,
	})
	dockMem := renderDashPanel(dashPanel{
		title:    "DOCKER RAM",
		subtitle: fmt.Sprintf("%d %s MiB · %d pts", dockerN, ctrLabel, n),
		values:   memVals,
		style:    lipgloss.NewStyle().Foreground(cDockMem),
		formatY:  formatMiBAxis,
		formatV:  formatMiBValue,
		fromZero: false,
		width:    panelW,
		height:   chartH,
	})

	diskW := panelW
	if width >= panelW*2+gap {
		diskW = panelW
	}
	disk := renderDashPanel(dashPanel{
		title:    "DISK",
		subtitle: fmt.Sprintf("used %% · %d pts", n),
		values:   m.graphsDisk.snapshot(),
		style:    lipgloss.NewStyle().Foreground(cDisk),
		formatY:  formatPctShort,
		formatV:  formatPctShort,
		fromZero: true,
		width:    diskW,
		height:   chartH,
	})

	var b strings.Builder
	b.WriteString(helpTitleStyle.Render(m.graphsTitle))
	src := m.graphsHostSrc
	if src == "" {
		src = "…"
	}
	b.WriteString("\n")
	b.WriteString(chartLabelStyle().Render(fmt.Sprintf(
		"overview · host:%s · conf: %d · poll: %s",
		src, statsHistCap, graphsSampleInterval,
	)))
	b.WriteString("\n\n")

	if width >= panelW*2+gap {
		b.WriteString(joinPanelRow(hostCPU, hostMem, gap))
		b.WriteString("\n\n")
		b.WriteString(joinPanelRow(dockCPU, dockMem, gap))
		b.WriteString("\n\n")
		b.WriteString(disk)
	} else {
		b.WriteString(hostCPU)
		b.WriteString("\n\n")
		b.WriteString(hostMem)
		b.WriteString("\n\n")
		b.WriteString(dockCPU)
		b.WriteString("\n\n")
		b.WriteString(dockMem)
		b.WriteString("\n\n")
		b.WriteString(disk)
	}
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
		BorderForeground(cBorder).
		Width(max(20, m.width-2))

	body := frame.Render(m.vp.View())
	help := renderHelpRow("graphs", []helpBinding{
		{"esc", "back"},
		{"r", "sample now"},
		{"q", "quit"},
	})
	status := statusStyle.Render(m.status)
	if m.errMsg != "" {
		status = errorStyle.Render("ERR: " + truncate(m.errMsg, m.width-10))
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, status, help)
}
