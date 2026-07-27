package ui

import (
	"context"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cooffeeRequired/dockafe/internal/docker"
)

const eventsHistCap = 200

type eventMsg struct {
	ev docker.EventInfo
}

type eventErrMsg struct {
	err error
}

type eventWatchStartedMsg struct {
	ch     <-chan docker.EventInfo
	errCh  <-chan error
	cancel context.CancelFunc
}

func startEventWatchCmd(client *docker.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		ch, errCh := client.WatchEvents(ctx)
		return eventWatchStartedMsg{ch: ch, errCh: errCh, cancel: cancel}
	}
}

func (m *Model) applyEventWatchStart(msg eventWatchStartedMsg) tea.Cmd {
	if m.eventCancel != nil {
		m.eventCancel()
	}
	m.eventCancel = msg.cancel
	m.eventCh = msg.ch
	m.eventErrCh = msg.errCh
	m.eventWatching = true
	return m.pollEventCmd()
}

func (m *Model) stopEventWatch() {
	if m.eventCancel != nil {
		m.eventCancel()
		m.eventCancel = nil
	}
	m.eventWatching = false
	m.eventCh = nil
	m.eventErrCh = nil
}

func (m Model) pollEventCmd() tea.Cmd {
	ch := m.eventCh
	errCh := m.eventErrCh
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		select {
		case err, ok := <-errCh:
			if ok && err != nil {
				return eventErrMsg{err: err}
			}
			select {
			case ev, ok := <-ch:
				if !ok {
					return nil
				}
				return eventMsg{ev: ev}
			default:
				return nil
			}
		case ev, ok := <-ch:
			if !ok {
				return nil
			}
			return eventMsg{ev: ev}
		}
	}
}

func (m *Model) appendEvent(ev docker.EventInfo) {
	m.eventLog = append(m.eventLog, ev)
	if len(m.eventLog) > eventsHistCap {
		m.eventLog = m.eventLog[len(m.eventLog)-eventsHistCap:]
	}
	if ev.Level == docker.EventCritLevel {
		m.eventAlert = docker.FormatEventLine(ev)
	}
}

func (m Model) openEvents() (tea.Model, tea.Cmd) {
	m.mode = ModeEvents
	m.detailTitle = "Docker events"
	m.status = "live container events · esc back"
	m.errMsg = ""
	m.eventAlert = ""
	m.relayout()
	m.vp.SetContent(m.renderEventsBody())
	m.vp.GotoBottom()
	if !m.eventWatching {
		return m, startEventWatchCmd(m.client)
	}
	return m, m.pollEventCmd()
}

func (m *Model) refreshEventsView() {
	if m.mode != ModeEvents {
		return
	}
	atBottom := m.vp.AtBottom()
	m.vp.SetContent(m.renderEventsBody())
	if atBottom {
		m.vp.GotoBottom()
	}
}

func (m Model) renderEventsBody() string {
	if len(m.eventLog) == 0 {
		return chartLabelStyle().Render("Waiting for container events (start/stop/die/oom/health)…")
	}
	var b strings.Builder
	b.WriteString(helpTitleStyle.Render("Container events"))
	b.WriteString("\n")
	b.WriteString(chartLabelStyle().Render("* critical · ! warn · last " + strconv.Itoa(len(m.eventLog)) + " / " + strconv.Itoa(eventsHistCap)))
	b.WriteString("\n\n")
	warnStyle := lipgloss.NewStyle().Foreground(cWarn)
	for _, ev := range m.eventLog {
		line := docker.FormatEventLine(ev)
		switch ev.Level {
		case docker.EventCritLevel:
			b.WriteString(errorStyle.Render(line))
		case docker.EventWarnLevel:
			b.WriteString(warnStyle.Render(line))
		default:
			b.WriteString(statusStyle.Render(line))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func (m Model) viewEvents() string {
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		titleStyle.Render(m.versionTitle()),
		"  ",
		activeTabStyle.Render(" EVENTS "),
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
	help := renderHelpRow("events", []helpBinding{
		{"esc", "back"},
		{"q", "list"},
	})
	status := statusStyle.Render(m.status)
	if m.errMsg != "" {
		status = errorStyle.Render("ERR: " + truncate(m.errMsg, m.width-10))
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, status, help)
}
