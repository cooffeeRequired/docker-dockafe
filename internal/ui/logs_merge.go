package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// palette for container name prefixes in shared logs
func containerStyle(name string) lipgloss.Style {
	h := 0
	for _, r := range name {
		h = (h*31 + int(r)) % len(containerColors)
	}
	if h < 0 {
		h = -h
	}
	return lipgloss.NewStyle().Foreground(containerColors[h%len(containerColors)]).Bold(true)
}

type logEntry struct {
	ts    time.Time
	label string
	line  string // original line without requiring timestamp parse success
	raw   string // full original line from docker
	order int    // stable sort
}

// mergeComposeLogs builds a shared timeline: each line tagged with container name,
// sorted by docker timestamp when available.
func mergeComposeLogs(names []string, bodies []string, errs []error) string {
	entries := make([]logEntry, 0, 256)
	order := 0

	for i, body := range bodies {
		label := shortContainerLabel(names, i)
		if i < len(errs) && errs[i] != nil {
			entries = append(entries, logEntry{
				ts:    time.Time{},
				label: label,
				line:  fmt.Sprintf("(error: %v)", errs[i]),
				raw:   "",
				order: order,
			})
			order++
			continue
		}
		for _, raw := range strings.Split(body, "\n") {
			if strings.TrimSpace(raw) == "" {
				continue
			}
			ts, rest := parseLogTimestamp(raw)
			entries = append(entries, logEntry{
				ts:    ts,
				label: label,
				line:  rest,
				raw:   raw,
				order: order,
			})
			order++
		}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if !a.ts.IsZero() && !b.ts.IsZero() && !a.ts.Equal(b.ts) {
			return a.ts.Before(b.ts)
		}
		if a.ts.IsZero() != b.ts.IsZero() {
			return !a.ts.IsZero() // dated lines first when mixed
		}
		return a.order < b.order
	})

	// pad labels to aligned column
	width := 0
	for _, e := range entries {
		if w := len([]rune(e.label)); w > width {
			width = w
		}
	}
	if width > 28 {
		width = 28
	}
	if width < 8 {
		width = 8
	}

	var b strings.Builder
	b.WriteString(logSepStyle.Render(fmt.Sprintf("── shared logs · %d services ──", len(names))))
	b.WriteByte('\n')

	for _, e := range entries {
		label := e.label
		runes := []rune(label)
		if len(runes) > width {
			label = string(runes[:width-1]) + "…"
		}
		pad := width - len([]rune(label))
		if pad < 0 {
			pad = 0
		}
		prefix := containerStyle(e.label).Render(label) + strings.Repeat(" ", pad) + logSepStyle.Render(" │ ")

		if e.raw != "" {
			// Keep original line content (timestamps + ANSI), just prefix container
			plain := e.raw
			if reHasANSI.MatchString(plain) {
				b.WriteString(prefix)
				b.WriteString(plain)
			} else {
				b.WriteString(prefix)
				b.WriteString(colorizePlainLogLine(plain))
			}
		} else {
			b.WriteString(prefix)
			b.WriteString(logErrStyle.Render(e.line))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func shortContainerLabel(names []string, i int) string {
	if i < 0 || i >= len(names) || names[i] == "" {
		return fmt.Sprintf("c%d", i+1)
	}
	name := names[i]
	parts := strings.Split(name, "-")
	// coffee-page-cms-1 → cms-1 ; dev-postgres → postgres (keep if short)
	if len(parts) >= 2 {
		last := parts[len(parts)-1]
		if isAllDigits(last) && len(parts) >= 2 {
			svc := parts[len(parts)-2] + "-" + last
			return svc
		}
	}
	if len(name) > 28 {
		return name[:27] + "…"
	}
	return name
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func parseLogTimestamp(line string) (time.Time, string) {
	plain := stripANSI(line)
	m := reTimestamp.FindStringSubmatch(plain)
	if m == nil {
		return time.Time{}, plain
	}
	tsRaw := m[1]
	rest := m[2]
	// Docker RFC3339Nano-ish
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05.999999999Z",
		"2006-01-02T15:04:05Z",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, tsRaw); err == nil {
			return t, rest
		}
	}
	return time.Time{}, plain
}
