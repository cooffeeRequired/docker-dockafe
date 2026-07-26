package docker

import "testing"

func TestCleanVolPath(t *testing.T) {
	cases := map[string]string{
		"":         "",
		".":        "",
		"/":        "",
		"a/b":      "a/b",
		"/a/b/":    "a/b",
		"a/../b":   "b",
		`a\b\c`:    "a/b/c",
		"//a//b//": "a/b",
	}
	for in, want := range cases {
		if got := CleanVolPath(in); got != want {
			t.Errorf("CleanVolPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestShellQuote(t *testing.T) {
	if got := ShellQuote("hello"); got != "'hello'" {
		t.Fatalf("got %q", got)
	}
	if got := ShellQuote("it's"); got != `'it'"'"'s'` {
		t.Fatalf("got %q", got)
	}
}
