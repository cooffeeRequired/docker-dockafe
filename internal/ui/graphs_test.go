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
