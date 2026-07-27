package ui

import (
	"strings"
	"testing"
)

func TestStatsSeriesPushCap(t *testing.T) {
	s := newStatsSeries(3)
	s.push(1, 10)
	s.push(2, 20)
	s.push(3, 30)
	s.push(4, 40)
	if s.len() != 3 {
		t.Fatalf("len=%d want 3", s.len())
	}
	last, ok := s.last()
	if !ok || last.cpu != 4 || last.mem != 40 {
		t.Fatalf("last=%v ok=%v", last, ok)
	}
	cpu := s.cpuValues()
	if len(cpu) != 3 || cpu[0] != 2 || cpu[2] != 4 {
		t.Fatalf("cpu=%v", cpu)
	}
}

func TestSparklineEmptyAndWidth(t *testing.T) {
	if got := sparkline(nil, 5); got != "·····" {
		t.Fatalf("empty sparkline=%q", got)
	}
	got := sparkline([]float64{0, 50, 100}, 3)
	if len([]rune(got)) != 3 {
		t.Fatalf("sparkline runes=%q", got)
	}
}

func TestBlockChartEmpty(t *testing.T) {
	got := blockChart(nil, 4, 2)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("lines=%d", len(lines))
	}
	for _, line := range lines {
		if line != "····" {
			t.Fatalf("line=%q", line)
		}
	}
}

func TestResampleSingle(t *testing.T) {
	got := resample([]float64{7}, 4)
	if len(got) != 4 {
		t.Fatalf("len=%d", len(got))
	}
	for _, v := range got {
		if v != 7 {
			t.Fatalf("got=%v", got)
		}
	}
}

func TestFormatMemBytes(t *testing.T) {
	if formatMemBytes(512) != "512B" {
		t.Fatalf("512: %s", formatMemBytes(512))
	}
	if formatMemBytes(2048) != "2.0KiB" {
		t.Fatalf("2048: %s", formatMemBytes(2048))
	}
}

func TestComposeHistKey(t *testing.T) {
	if composeHistKey("git") != "compose:git" {
		t.Fatal(composeHistKey("git"))
	}
}

func TestAreaChartHasAxis(t *testing.T) {
	got := areaChart([]float64{1, 2, 3, 4, 8}, 40, 6, true, formatCPUAxis, chartCPUStyle())
	if !strings.Contains(got, "│") {
		t.Fatalf("expected Y-axis, got:\n%s", got)
	}
	lines := strings.Split(got, "\n")
	if len(lines) != 6 {
		t.Fatalf("height=%d want 6", len(lines))
	}
}

func TestScaleRangeFromZero(t *testing.T) {
	minV, maxV := scaleRange([]float64{2, 4}, true)
	if minV != 0 || maxV < 4 {
		t.Fatalf("min=%v max=%v", minV, maxV)
	}
}

func TestAvgDelta(t *testing.T) {
	if avg([]float64{2, 4, 6}) != 4 {
		t.Fatal(avg([]float64{2, 4, 6}))
	}
	if delta([]float64{2, 4, 6}) != 4 {
		t.Fatal(delta([]float64{2, 4, 6}))
	}
}

func TestMetricSeriesPushCap(t *testing.T) {
	s := newMetricSeries(2)
	s.push(1)
	s.push(2)
	s.push(3)
	if s.len() != 2 {
		t.Fatalf("len=%d", s.len())
	}
	last, ok := s.last()
	if !ok || last != 3 {
		t.Fatalf("last=%v ok=%v", last, ok)
	}
}

func TestPanelBarChartGuides(t *testing.T) {
	got := panelBarChart([]float64{1, 2, 4, 8}, 36, 6, true, formatPctShort, chartCPUStyle())
	if !strings.Contains(got, "┊") {
		t.Fatalf("expected axis, got:\n%s", got)
	}
	if !strings.Contains(got, "┈") {
		t.Fatalf("expected dotted guides, got:\n%s", got)
	}
}

func TestRenderDashPanelFooter(t *testing.T) {
	got := renderDashPanel(dashPanel{
		title:    "HOST CPU",
		subtitle: "load % · 3 pts",
		values:   []float64{1, 2, 3},
		style:    chartCPUStyle(),
		formatY:  formatPctShort,
		formatV:  formatPctShort,
		fromZero: true,
		width:    40,
		height:   5,
	})
	if !strings.Contains(got, "HOST CPU") {
		t.Fatal(got)
	}
	if !strings.Contains(got, "now") || !strings.Contains(got, "min") || !strings.Contains(got, "max") {
		t.Fatalf("footer missing:\n%s", got)
	}
}

func TestJoinPanelRow(t *testing.T) {
	got := joinPanelRow("a\nb", "c\nd", 2)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("lines=%d got=%q", len(lines), got)
	}
}
