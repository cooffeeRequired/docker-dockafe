package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const splashMinDuration = 800 * time.Millisecond

// Classic Docker whale (terminal ASCII) + coffee-tui wordmark.
var dockerWhale = strings.TrimRight(`
                     ##         .
               ## ## ##        ==
            ## ## ## ## ##    ===
        /"""""""""""""""""""\___/ ===
   ~~~ {~~ ~~~~ ~~~ ~~~~ ~~~ ~ /  ===- ~~~
        \______ o           __/
          \    \         __/
           \____\_______/
`, "\n")

var coffeeMark = strings.TrimRight(`
      )  (
     (   ) )
      ) ( (
    _______)_
 .-'---------|  
( C|/\/\/\/\/|
 '-./\/\/\/\/|
   '_________'
    '-------'
`, "\n")

type splashReadyMsg struct{}
type splashAnimMsg struct{}

func splashTimerCmd() tea.Cmd {
	return tea.Tick(splashMinDuration, func(time.Time) tea.Msg {
		return splashReadyMsg{}
	})
}

func splashAnimCmd() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
		return splashAnimMsg{}
	})
}

func (m *Model) tryLeaveSplash() tea.Cmd {
	if m.mode != ModeSplash {
		return nil
	}
	if !m.splashMinDone || !m.splashDataReady {
		return nil
	}
	m.mode = ModeList
	m.status = fmt.Sprintf("sync %s · sort %s", m.lastSync.Format("15:04:05"), m.sortLabel())
	m.relayout()
	m.applyRows()
	return nil
}

func (m Model) viewSplash() string {
	whaleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Bold(true)
	coffeeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("180"))
	brandStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("229")).
		Bold(true)
	tagStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("110"))
	mutedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("243"))
	barStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39"))
	errStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true)

	whale := whaleStyle.Render(dockerWhale)
	coffee := coffeeStyle.Render(coffeeMark)

	var art string
	if m.width >= 78 {
		art = lipgloss.JoinHorizontal(lipgloss.Bottom, whale, "   ", coffee)
	} else {
		art = lipgloss.JoinVertical(lipgloss.Center, whale, "", coffee)
	}

	brand := brandStyle.Render(AppName) + "  " + mutedStyle.Render("v"+AppVersion)
	tagline := tagStyle.Render("containers · compose · control")
	byline := mutedStyle.Render("Docker + café — brewed for the terminal")
	if m.updateAvailable {
		byline = updateBadgeStyle.Render("update " + m.updateLatest + " available · press U")
	}

	var statusLine string
	switch {
	case m.errMsg != "" && m.splashDataReady:
		statusLine = errStyle.Render("Docker API error — press Enter to continue")
	case m.splashDataReady && m.splashMinDone:
		statusLine = mutedStyle.Render("ready")
	case m.splashDataReady:
		statusLine = barStyle.Render("connected") + mutedStyle.Render("  · warming up…")
	default:
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		spin := frames[int(time.Now().UnixNano()/1e8)%len(frames)]
		statusLine = barStyle.Render(spin) + " " + mutedStyle.Render("connecting to Docker…")
	}

	hint := mutedStyle.Render("Enter skip · q quit")
	if !m.splashDataReady {
		hint = mutedStyle.Render("q quit")
	}

	block := lipgloss.JoinVertical(lipgloss.Center,
		art,
		"",
		brand,
		tagline,
		byline,
		"",
		statusLine,
		hint,
	)

	w, h := m.width, m.height
	if w < 1 {
		w = 80
	}
	if h < 1 {
		h = 24
	}
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, block)
}
