package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var sparkBlocks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

func sparkline(values []float64, width int) string {
	if width < 1 {
		return ""
	}
	if len(values) == 0 {
		return strings.Repeat("·", width)
	}
	vals := resample(values, width)
	minV, maxV := minMax(vals)
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
	_, maxV := minMax(vals)
	if maxV <= 0 {
		maxV = 1
	}

	grid := make([][]rune, height)
	for y := 0; y < height; y++ {
		grid[y] = make([]rune, width)
		for x := 0; x < width; x++ {
			grid[y][x] = ' '
		}
	}

	for x, v := range vals {
		level := int(math.Round(v / maxV * float64(height)))
		if level < 0 {
			level = 0
		}
		if level > height {
			level = height
		}
		for y := 0; y < level; y++ {
			grid[height-1-y][x] = '█'
		}
	}

	lines := make([]string, height)
	for y := 0; y < height; y++ {
		lines[y] = string(grid[y])
	}
	return strings.Join(lines, "\n")
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

func chartLabelStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
}

func chartValueStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true)
}

func chartBlockStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
}
