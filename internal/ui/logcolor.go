package ui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	logTSStyle    = lipgloss.NewStyle().Foreground(cLogTS)
	logErrStyle   = lipgloss.NewStyle().Foreground(cError).Bold(true)
	logWarnStyle  = lipgloss.NewStyle().Foreground(cWarn)
	logInfoStyle  = lipgloss.NewStyle().Foreground(cOK)
	logDebugStyle = lipgloss.NewStyle().Foreground(cLogDebug)
	logMsgStyle   = lipgloss.NewStyle().Foreground(cLogMsg)
	logSepStyle   = lipgloss.NewStyle().Foreground(cFaint)
)

var (
	reTimestamp = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T[0-9:.+\-Z]+)\s*(.*)$`)
	reLevel     = regexp.MustCompile(`(?i)\b(error|err|fatal|panic|warn(?:ing)?|info|debug|trace)\b`)
	reHasANSI   = regexp.MustCompile(`\x1b\[[0-9;]*m`)
)

// colorizeLogs keeps existing ANSI from containers and adds colors for
// timestamps / log levels on plain lines.
func colorizeLogs(raw string) string {
	if raw == "" {
		return raw
	}
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			out = append(out, line)
			continue
		}
		if strings.HasPrefix(line, "────────") {
			out = append(out, logSepStyle.Render(line))
			continue
		}
		// Preserve container ANSI as-is (still lightly tint timestamp if present)
		if reHasANSI.MatchString(line) {
			out = append(out, colorizeTimestampOnly(line))
			continue
		}
		out = append(out, colorizePlainLogLine(line))
	}
	return strings.Join(out, "\n")
}

func colorizeTimestampOnly(line string) string {
	m := reTimestamp.FindStringSubmatch(stripANSI(line))
	if m == nil {
		return line
	}
	// If original has ANSI, don't double-style — return original
	return line
}

func colorizePlainLogLine(line string) string {
	m := reTimestamp.FindStringSubmatch(line)
	var ts, rest string
	if m != nil {
		ts = m[1]
		rest = m[2]
	} else {
		rest = line
	}

	styledRest := styleByLevel(rest)
	if ts != "" {
		return logTSStyle.Render(ts) + " " + styledRest
	}
	return styledRest
}

func styleByLevel(msg string) string {
	loc := reLevel.FindStringIndex(msg)
	if loc == nil {
		return logMsgStyle.Render(msg)
	}
	level := msg[loc[0]:loc[1]]
	var levelStyle lipgloss.Style
	switch strings.ToLower(level) {
	case "error", "err", "fatal", "panic":
		levelStyle = logErrStyle
	case "warn", "warning":
		levelStyle = logWarnStyle
	case "info":
		levelStyle = logInfoStyle
	case "debug", "trace":
		levelStyle = logDebugStyle
	default:
		levelStyle = logMsgStyle
	}
	return logMsgStyle.Render(msg[:loc[0]]) +
		levelStyle.Render(level) +
		logMsgStyle.Render(msg[loc[1]:])
}

func stripANSI(s string) string {
	return reHasANSI.ReplaceAllString(s, "")
}
