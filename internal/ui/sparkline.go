package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var sparkBlocks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// eighthBlocks maps 0..8 fill levels inside one terminal cell.
var eighthBlocks = []rune{' ', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

func sparkline(values []float64, width int) string {
	if width < 1 {
		return ""
	}
	if len(values) == 0 {
		return strings.Repeat("·", width)
	}
	vals := resample(values, width)
	minV, maxV := scaleRange(vals, false)
	span := maxV - minV
	if span <= 0 {
		span = 1
	}
	var b strings.Builder
	b.Grow(width * 3)
	for _, v := range vals {
		idx := int(math.Round((v - minV) / span * float64(len(sparkBlocks)-1)))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkBlocks) {
			idx = len(sparkBlocks) - 1
		}
		b.WriteRune(sparkBlocks[idx])
	}
	return b.String()
}

// areaChart draws a filled area plot with Y-axis labels and sub-cell vertical resolution.
func areaChart(values []float64, width, height int, fromZero bool, formatY func(float64) string, barStyle lipgloss.Style) string {
	if width < 8 || height < 2 {
		return ""
	}
	labelW := 8
	plotW := width - labelW - 1
	if plotW < 4 {
		plotW = width
		labelW = 0
	}

	if len(values) == 0 {
		lines := make([]string, height)
		empty := chartMutedStyle().Render(strings.Repeat("·", max(4, plotW)))
		for i := range lines {
			if labelW > 0 {
				lines[i] = strings.Repeat(" ", labelW) + chartMutedStyle().Render("│") + empty
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

	grid := make([][]rune, height)
	for y := 0; y < height; y++ {
		grid[y] = make([]rune, plotW)
		guide := y == height/2
		for x := 0; x < plotW; x++ {
			topEighth := (height - y) * 8
			bottomEighth := (height - 1 - y) * 8
			lv := levels[x]
			switch {
			case lv <= bottomEighth:
				if guide {
					grid[y][x] = '─'
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
		plot := barStyle.Render(string(grid[y]))
		if labelW <= 0 {
			lines[y] = plot
			continue
		}
		var label string
		switch y {
		case 0:
			label = formatY(maxV)
		case height - 1:
			label = formatY(minV)
		case height / 2:
			label = formatY(minV + span/2)
		default:
			label = ""
		}
		if len([]rune(label)) > labelW {
			label = truncateRunes(label, labelW)
		}
		pad := labelW - len([]rune(label))
		lines[y] = chartMutedStyle().Render(strings.Repeat(" ", pad)+label) +
			chartMutedStyle().Render("│") + plot
	}
	return strings.Join(lines, "\n")
}

func scaleRange(values []float64, fromZero bool) (float64, float64) {
	minV, maxV := minMax(values)
	if fromZero {
		minV = 0
	}
	if maxV <= minV {
		if fromZero {
			return 0, 1
		}
		pad := math.Abs(maxV) * 0.1
		if pad < 0.01 {
			pad = 0.01
		}
		return minV - pad, maxV + pad
	}
	pad := (maxV - minV) * 0.12
	if fromZero {
		maxV = maxV + pad
		if maxV < 1 {
			maxV = math.Max(maxV, 1)
		}
		return 0, maxV
	}
	minV = minV - pad
	if minV < 0 && valuesLookNonNegative(values) {
		minV = 0
	}
	return minV, maxV + pad
}

func valuesLookNonNegative(values []float64) bool {
	for _, v := range values {
		if v < 0 {
			return false
		}
	}
	return true
}

func avg(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func delta(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	return values[len(values)-1] - values[0]
}

func resample(values []float64, width int) []float64 {
	if width <= 0 {
		return nil
	}
	if len(values) == 0 {
		return make([]float64, width)
	}
	if len(values) == width {
		out := make([]float64, width)
		copy(out, values)
		return out
	}
	out := make([]float64, width)
	if len(values) == 1 {
		for i := range out {
			out[i] = values[0]
		}
		return out
	}
	last := float64(len(values) - 1)
	for i := 0; i < width; i++ {
		pos := float64(i) * last / float64(width-1)
		lo := int(math.Floor(pos))
		hi := int(math.Ceil(pos))
		if hi >= len(values) {
			hi = len(values) - 1
		}
		if lo == hi {
			out[i] = values[lo]
			continue
		}
		frac := pos - float64(lo)
		out[i] = values[lo]*(1-frac) + values[hi]*frac
	}
	return out
}

func minMax(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	minV, maxV := values[0], values[0]
	for _, v := range values[1:] {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	return minV, maxV
}

func formatMemBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := uint64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func formatCPUAxis(v float64) string {
	if v >= 100 {
		return fmt.Sprintf("%.0f%%", v)
	}
	if v >= 10 {
		return fmt.Sprintf("%.1f%%", v)
	}
	return fmt.Sprintf("%.2f%%", v)
}

func formatMemAxis(v float64) string {
	if v < 0 {
		v = 0
	}
	return formatMemBytes(uint64(v))
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

func chartLabelStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(cMuted)
}

func chartMutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(cFaint)
}

func chartValueStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(cBright).Bold(true)
}

func chartCPUStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(cCPU)
}

func chartMemStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(cMem)
}

func chartSectionStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(cBright).Bold(true)
}

func blockChart(values []float64, width, height int) string {
	if width < 1 || height < 1 {
		return ""
	}
	if len(values) == 0 {
		lines := make([]string, height)
		for i := range lines {
			lines[i] = strings.Repeat("·", width)
		}
		return strings.Join(lines, "\n")
	}
	vals := resample(values, width)
	_, maxV := scaleRange(vals, true)
	span := maxV
	if span <= 0 {
		span = 1
	}
	grid := make([][]rune, height)
	for y := 0; y < height; y++ {
		grid[y] = make([]rune, width)
		for x := 0; x < width; x++ {
			grid[y][x] = ' '
		}
	}
	for x, v := range vals {
		level := int(math.Round(v / span * float64(height*8)))
		for y := 0; y < height; y++ {
			bottom := (height - 1 - y) * 8
			top := (height - y) * 8
			switch {
			case level <= bottom:
				grid[y][x] = ' '
			case level >= top:
				grid[y][x] = '█'
			default:
				grid[y][x] = eighthBlocks[level-bottom]
			}
		}
	}
	lines := make([]string, height)
	for y := 0; y < height; y++ {
		lines[y] = string(grid[y])
	}
	return strings.Join(lines, "\n")
}
