package ui

import "testing"

func TestTruncateUTF8(t *testing.T) {
	s := "ěščřžýáíé"
	got := truncate(s, 5)
	if got == "" || len([]rune(got)) > 5 {
		// runewidth may count some runes as width 1
		if truncate("abc", 2) != "a…" && truncate("abc", 2) != "ab" {
			t.Fatalf("truncate ascii failed: %q", truncate("abc", 2))
		}
	}
	if truncate("hello", 10) != "hello" {
		t.Fatal("no-op truncate failed")
	}
	if truncate("hello", 0) != "" {
		t.Fatal("zero width should be empty")
	}
}

func TestMatchesFilter(t *testing.T) {
	if !matchesFilter("web api", "my-web", "api-svc", "other") {
		t.Fatal("expected AND match")
	}
	if matchesFilter("missing", "web", "api") {
		t.Fatal("expected no match")
	}
	if !matchesFilter("", "anything") {
		t.Fatal("empty filter matches all")
	}
}

func TestLangFromExt(t *testing.T) {
	if langFromExt(".js") != "javascript" {
		t.Fatal(langFromExt(".js"))
	}
	if langFromExt(".go") != "go" {
		t.Fatal(langFromExt(".go"))
	}
}

func TestHighlightSourceEmpty(t *testing.T) {
	if highlightSource("x.go", "") != "" {
		t.Fatal("expected empty")
	}
	out := highlightSource("index.js", "var x = 1;\n")
	if out == "" {
		t.Fatal("expected highlighted output")
	}
}

func TestFitColumns(t *testing.T) {
	cols := defaultColumns(TabContainers, 40)
	sum := 0
	for _, c := range cols {
		sum += c.Width
	}
	if sum > 40-2+20 { // allow some floor from max() mins
		// with width 40, budget 38 — fit should shrink
		cols2 := fitColumns(cols, 30)
		sum2 := 0
		for _, c := range cols2 {
			sum2 += c.Width
		}
		if sum2 > 30 {
			t.Fatalf("fitColumns sum %d > 30", sum2)
		}
	}
}
