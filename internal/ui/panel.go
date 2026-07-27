package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type dashPanel struct {
	title    string
	subtitle string
	values   []float64
	style    lipgloss.Style
	formatY  func(float64) string
	formatV  func(float64) string
	fromZero bool
	width    int
	height   int
}

func renderDashPanel(p dashPanel) string {
	if p.width < 16 {
		p.width = 16
	}
	if p.height < 4 {
		p.height = 4
	}

	var b strings.Builder
	bar := p.style.Bold(true).Render("┃")
	title := p.style.Bold(true).Render(" " + p.title)
	b.WriteString(bar + title)
	b.WriteByte('\n')
	b.WriteString(chartMutedStyle().Render("  " + p.subtitle))
	b.WriteByte('\n')

	chartW := p.width
	chartH := p.height
	b.WriteString(panelBarChart(p.values, chartW, chartH, p.fromZero, p.formatY, p.style))
	b.WriteByte('\n')

	if len(p.values) == 0 {
		b.WriteString(chartMutedStyle().Render("  waiting…"))
		return b.String()
	}

	now := p.values[len(p.values)-1]
	minV, maxV := minMax(p.values)
	sparkW := min(14, max(6, p.width/4))
	footer := fmt.Sprintf("  now %s  min %s  max %s  ",
		p.formatV(now), p.formatV(minV), p.formatV(maxV))
	spark := p.style.Render(sparkline(p.values, sparkW))
	b.WriteString(chartLabelStyle().Render(footer) + spark)
	return b.String()
}

// panelBarChart draws a btop-style bar plot with dotted horizontal guides.
func panelBarChart(values []float64, width, height int, fromZero bool, formatY func(float64) string, barStyle lipgloss.Style) string {
	if width < 10 || height < 2 {
		return ""
	}
	labelW := 6
	plotW := width - labelW - 1
	if plotW < 6 {
		plotW = max(4, width-2)
		labelW = width - plotW - 1
		if labelW < 0 {
			labelW = 0
		}
	}

	if len(values) == 0 {
		lines := make([]string, height)
		empty := chartMutedStyle().Render(strings.Repeat("·", plotW))
		for i := range lines {
			if labelW > 0 {
				lines[i] = strings.Repeat(" ", labelW) + chartMutedStyle().Render("┊") + empty
			} else {
				lines[i] = empty
			}
		}
		return strings.Join(lines, "\n")
	}

	vals := resample(values, plotW)
	minV, maxV := scaleRange(vals, fromZero)
	span := maxV - minV
	if span <= 0 {
		span = 1
	}

	levels := make([]int, plotW)
	for i, v := range vals {
		frac := (v - minV) / span
		if frac < 0 {
			frac = 0
		}
		if frac > 1 {
			frac = 1
		}
		levels[i] = int(math.Round(frac * float64(height*8)))
	}

	// Guide rows: top, mid, bottom get dotted background where empty.
	guideRows := map[int]bool{0: true, height / 2: true, height - 1: true}

	grid := make([][]rune, height)
	for y := 0; y < height; y++ {
		grid[y] = make([]rune, plotW)
		for x := 0; x < plotW; x++ {
			topEighth := (height - y) * 8
			bottomEighth := (height - 1 - y) * 8
			lv := levels[x]
			switch {
			case lv <= bottomEighth:
				if guideRows[y] {
					grid[y][x] = '┈'
				} else {
					grid[y][x] = ' '
				}
			case lv >= topEighth:
				grid[y][x] = '█'
			default:
				grid[y][x] = eighthBlocks[lv-bottomEighth]
			}
		}
	}

	lines := make([]string, height)
	for y := 0; y < height; y++ {
		raw := string(grid[y])
		// Color bars; keep guide dots muted.
		plot := colorPlotRow(raw, barStyle)
		if labelW <= 0 {
			lines[y] = plot
			continue
		}
		var label string
		switch {
		case y == 0:
			label = formatY(maxV)
		case y == height-1:
			label = formatY(minV)
		case y == height/2:
			label = formatY(minV + span/2)
		}
		if len([]rune(label)) > labelW {
			label = truncateRunes(label, labelW)
		}
		pad := labelW - len([]rune(label))
		lines[y] = chartMutedStyle().Render(strings.Repeat(" ", pad)+label) +
			chartMutedStyle().Render("┊") + plot
	}
	return strings.Join(lines, "\n")
}

func colorPlotRow(row string, barStyle lipgloss.Style) string {
	var b strings.Builder
	runes := []rune(row)
	i := 0
	for i < len(runes) {
		r := runes[i]
		if r == '┈' || r == ' ' {
			j := i
			for j < len(runes) && (runes[j] == '┈' || runes[j] == ' ') {
				j++
			}
			chunk := string(runes[i:j])
			b.WriteString(chartMutedStyle().Render(chunk))
			i = j
			continue
		}
		j := i
		for j < len(runes) && runes[j] != '┈' && runes[j] != ' ' {
			j++
		}
		b.WriteString(barStyle.Render(string(runes[i:j])))
		i = j
	}
	return b.String()
}

func formatPctShort(v float64) string {
	if v >= 100 {
		return fmt.Sprintf("%.0f", v)
	}
	if v >= 10 {
		return fmt.Sprintf("%.1f", v)
	}
	return fmt.Sprintf("%.2f", v)
}

func formatMiBAxis(v float64) string {
	if v < 0 {
		v = 0
	}
	return fmt.Sprintf("%.0f", v/1024/1024)
}

func formatMiBValue(v float64) string {
	if v < 0 {
		v = 0
	}
	return fmt.Sprintf("%.0f", v/1024/1024)
}

func joinPanelRow(left, right string, gap int) string {
	if gap < 1 {
		gap = 2
	}
	lLines := strings.Split(left, "\n")
	rLines := strings.Split(right, "\n")
	n := max(len(lLines), len(rLines))
	leftW := 0
	for _, line := range lLines {
		w := lipgloss.Width(line)
		if w > leftW {
			leftW = w
		}
	}
	var b strings.Builder
	pad := strings.Repeat(" ", gap)
	for i := 0; i < n; i++ {
		var l, r string
		if i < len(lLines) {
			l = lLines[i]
		}
		if i < len(rLines) {
			r = rLines[i]
		}
		extra := leftW - lipgloss.Width(l)
		if extra < 0 {
			extra = 0
		}
		b.WriteString(l)
		b.WriteString(strings.Repeat(" ", extra))
		b.WriteString(pad)
		b.WriteString(r)
		if i < n-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
