package ui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	logSearchHitStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("62")).
				Bold(true)
	logSearchBarStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("236")).
				Padding(0, 1)
)

func (m *Model) initLogSearch() {
	ti := textinput.New()
	ti.CharLimit = 200
	ti.Width = max(20, m.width-30)
	ti.Prompt = ""
	m.logsSearchInput = ti
}

func (m *Model) openLogSearch(regex bool) {
	if m.logsSearchInput.Value() == "" && m.logsSearchQuery != "" {
		// keep previous query
	}
	if m.logsSearchInput.Width == 0 {
		m.initLogSearch()
	}
	m.logsSearchOpen = true
	m.logsSearchRegex = regex
	m.logsFollow = false
	m.logsSearchInput.Width = max(20, m.width-36)
	if regex {
		m.logsSearchInput.Placeholder = "regex…"
		m.logsSearchInput.Prompt = "regex> "
	} else {
		m.logsSearchInput.Placeholder = "find…"
		m.logsSearchInput.Prompt = "find> "
	}
	if m.logsSearchQuery != "" {
		m.logsSearchInput.SetValue(m.logsSearchQuery)
	}
	m.logsSearchInput.Focus()
	m.logsSearchInput.CursorEnd()
	m.status = "searching logs · Enter find · n/N next/prev · esc close"
}

func (m *Model) closeLogSearch() {
	m.logsSearchOpen = false
	m.logsSearchInput.Blur()
	m.logsSearchMatches = nil
	m.logsSearchIdx = 0
	// restore full content without hit highlighting
	if m.detailBody != "" {
		m.vp.SetContent(m.detailBody)
	}
	m.status = "search closed"
}

func (m *Model) applyLogSearch() {
	q := strings.TrimSpace(m.logsSearchInput.Value())
	m.logsSearchQuery = q
	m.logsSearchMatches = nil
	m.logsSearchIdx = 0
	m.logsSearchErr = ""

	if q == "" {
		m.vp.SetContent(m.detailBody)
		m.status = "empty query"
		return
	}

	lines := strings.Split(m.detailBody, "\n")
	var re *regexp.Regexp
	if m.logsSearchRegex {
		var err error
		re, err = regexp.Compile("(?i)" + q)
		if err != nil {
			m.logsSearchErr = err.Error()
			m.status = "invalid regex: " + err.Error()
			return
		}
	}

	matches := make([]int, 0)
	for i, line := range lines {
		plain := stripANSI(line)
		ok := false
		if m.logsSearchRegex {
			ok = re.MatchString(plain)
		} else {
			ok = strings.Contains(strings.ToLower(plain), strings.ToLower(q))
		}
		if ok {
			matches = append(matches, i)
		}
	}
	m.logsSearchMatches = matches
	if len(matches) == 0 {
		m.vp.SetContent(m.detailBody)
		m.status = "0 matches"
		return
	}

	m.logsSearchIdx = 0
	m.renderLogSearchView()
	m.jumpToSearchMatch()
	mode := "text"
	if m.logsSearchRegex {
		mode = "regex"
	}
	m.status = fmt.Sprintf("%s · %d/%d · n/N next · Enter again", mode, 1, len(matches))
}

func (m *Model) renderLogSearchView() {
	if m.detailBody == "" {
		return
	}
	lines := strings.Split(m.detailBody, "\n")
	matchSet := map[int]struct{}{}
	for _, idx := range m.logsSearchMatches {
		matchSet[idx] = struct{}{}
	}
	q := m.logsSearchQuery
	var re *regexp.Regexp
	if m.logsSearchRegex && q != "" {
		re, _ = regexp.Compile("(?i)" + q)
	}

	out := make([]string, 0, len(lines))
	for i, line := range lines {
		if _, hit := matchSet[i]; !hit {
			out = append(out, line)
			continue
		}
		out = append(out, highlightLogLine(line, q, re, i == m.currentSearchLine()))
	}
	m.vp.SetContent(strings.Join(out, "\n"))
}

func (m Model) currentSearchLine() int {
	if len(m.logsSearchMatches) == 0 {
		return -1
	}
	if m.logsSearchIdx < 0 || m.logsSearchIdx >= len(m.logsSearchMatches) {
		return -1
	}
	return m.logsSearchMatches[m.logsSearchIdx]
}

func highlightLogLine(line, q string, re *regexp.Regexp, current bool) string {
	plain := stripANSI(line)
	bg := lipgloss.Color("238")
	if current {
		bg = lipgloss.Color("62")
	}
	lineStyle := lipgloss.NewStyle().Background(bg)

	// Highlight substrings inside plain text, then wrap whole line.
	if re != nil {
		plain = re.ReplaceAllStringFunc(plain, func(s string) string {
			return logSearchHitStyle.Render(s)
		})
	} else if q != "" {
		plain = highlightPlainIgnoreCase(plain, q)
	}
	marker := "  "
	if current {
		marker = "▶ "
	}
	return lineStyle.Render(marker+plain) + lipgloss.NewStyle().UnsetBackground().Render("")
}

func highlightPlainIgnoreCase(s, q string) string {
	if q == "" {
		return s
	}
	lower := strings.ToLower(s)
	ql := strings.ToLower(q)
	var b strings.Builder
	i := 0
	for {
		j := strings.Index(lower[i:], ql)
		if j < 0 {
			b.WriteString(s[i:])
			break
		}
		j += i
		b.WriteString(s[i:j])
		b.WriteString(logSearchHitStyle.Render(s[j : j+len(q)]))
		i = j + len(q)
	}
	return b.String()
}

func (m *Model) jumpToSearchMatch() {
	line := m.currentSearchLine()
	if line < 0 {
		return
	}
	// Place match near top third of viewport
	offset := line - m.vp.Height/3
	if offset < 0 {
		offset = 0
	}
	m.vp.SetYOffset(offset)
}

func (m *Model) nextSearchMatch(prev bool) {
	if len(m.logsSearchMatches) == 0 {
		m.status = "no matches — Enter to search"
		return
	}
	if prev {
		m.logsSearchIdx--
		if m.logsSearchIdx < 0 {
			m.logsSearchIdx = len(m.logsSearchMatches) - 1
		}
	} else {
		m.logsSearchIdx++
		if m.logsSearchIdx >= len(m.logsSearchMatches) {
			m.logsSearchIdx = 0
		}
	}
	m.renderLogSearchView()
	m.jumpToSearchMatch()
	m.status = fmt.Sprintf("match %d/%d", m.logsSearchIdx+1, len(m.logsSearchMatches))
}

func (m Model) handleLogSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.closeLogSearch()
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "enter":
		m.applyLogSearch()
		m.logsSearchInput.Blur()
		return m, nil
	case "ctrl+f", "alt+f":
		m.openLogSearch(false)
		return m, nil
	case "ctrl+g", "ctrl+shift+f", "alt+shift+f":
		m.openLogSearch(true)
		return m, nil
	case "up", "down", "ctrl+up", "ctrl+down", "pgup", "pgdown", "ctrl+u", "ctrl+d":
		// allow scrolling while search bar open
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}
	var cmd tea.Cmd
	m.logsSearchInput, cmd = m.logsSearchInput.Update(msg)
	return m, cmd
}

func (m Model) logSearchBarView() string {
	if !m.logsSearchOpen {
		return ""
	}
	kind := "FIND"
	if m.logsSearchRegex {
		kind = "REGEX"
	}
	info := ""
	if len(m.logsSearchMatches) > 0 {
		info = fmt.Sprintf("  %d/%d", m.logsSearchIdx+1, len(m.logsSearchMatches))
	} else if m.logsSearchQuery != "" && m.logsSearchErr == "" && !m.logsSearchInput.Focused() {
		info = "  0 hits"
	}
	if m.logsSearchErr != "" {
		info = "  ERR"
	}
	return logSearchBarStyle.Render(fmt.Sprintf(" %s %s%s ", kind, m.logsSearchInput.View(), info))
}
