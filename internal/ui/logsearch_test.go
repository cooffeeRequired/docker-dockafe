package ui

import (
	"strings"
	"testing"
)

func TestCompileLogRegex(t *testing.T) {
	re, err := compileLogRegex("error|warn")
	if err != nil {
		t.Fatal(err)
	}
	if !re.MatchString("ERROR found") {
		t.Fatal("expected case-insensitive match")
	}
	if _, err := compileLogRegex(""); err == nil {
		t.Fatal("expected empty pattern error")
	}
}

func TestTruncateForLogMatch(t *testing.T) {
	long := strings.Repeat("a", maxLogMatchBytes+50)
	got := truncateForLogMatch(long)
	if len(got) != maxLogMatchBytes {
		t.Fatalf("len=%d", len(got))
	}
}
