package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsDefaultRemoteReadOnly(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfg, err := os.UserConfigDir()
	if err != nil || cfg != dir {
		t.Skipf("UserConfigDir=%q XDG=%q", cfg, dir)
	}

	s, err := LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !s.IsRemoteReadOnly() {
		t.Fatal("default should be remote read-only")
	}

	s, err = SetRemoteReadOnly(false)
	if err != nil {
		t.Fatal(err)
	}
	if s.IsRemoteReadOnly() {
		t.Fatal("expected unlocked")
	}
	s2, err := LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s2.IsRemoteReadOnly() {
		t.Fatal("persist unlock failed")
	}
}

func TestAuditAppendAndTail(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfg, err := os.UserConfigDir()
	if err != nil || cfg != dir {
		t.Skipf("UserConfigDir=%q XDG=%q", cfg, dir)
	}

	if err := Audit("ssh://root@x", "remove_volume", "vol1", true, ""); err != nil {
		t.Fatal(err)
	}
	if err := Audit("ssh://root@x", "remove_volume", "vol2", false, "denied"); err != nil {
		t.Fatal(err)
	}
	path, err := AuditPath()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	tail, err := AuditTail(10)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tail, "action=remove_volume") || !strings.Contains(tail, "ok=false") {
		t.Fatalf("tail=%q", tail)
	}
	if !strings.Contains(tail, filepath.Base(path)) && !strings.Contains(tail, "audit.log") {
		t.Fatalf("expected path mention in tail header: %q", tail)
	}
}
