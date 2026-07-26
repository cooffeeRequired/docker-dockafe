package docker

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestResolveContainedHostPathRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "escape")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatal(err)
	}

	if _, err := resolveContainedHostPath(root, "escape"); err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}

	safe := filepath.Join(root, "ok.txt")
	if err := os.WriteFile(safe, []byte("yes"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := resolveContainedHostPath(root, "ok.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got != safe {
		// EvalSymlinks may rewrite path; ensure under root
		if !pathIsUnder(root, got) {
			t.Fatalf("resolved %q not under %q", got, root)
		}
	}
}

func TestResolveContainedHostPathAllowsNewFile(t *testing.T) {
	root := t.TempDir()
	got, err := resolveContainedHostPath(root, "new/dir/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !pathIsUnder(root, got) {
		t.Fatalf("%q not under %q", got, root)
	}
}

func TestValidateVolumeName(t *testing.T) {
	if err := validateVolumeName("ok_vol.1"); err != nil {
		t.Fatal(err)
	}
	if err := validateVolumeName("bad:name"); err == nil {
		t.Fatal("expected error")
	}
	if err := validateVolumeName("bad/name"); err == nil {
		t.Fatal("expected error")
	}
}
