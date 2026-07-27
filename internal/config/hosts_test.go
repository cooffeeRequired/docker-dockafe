package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUpsertAndRemoveHost(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	// UserConfigDir on Linux uses XDG_CONFIG_HOME when set.
	if got, err := Dir(); err != nil || got != filepath.Join(dir, "dockafe") {
		// macOS/Windows may ignore XDG; force via override by writing through HostsPath after chdir-like stub.
		// Fallback: skip if UserConfigDir doesn't honor XDG.
		cfg, err := os.UserConfigDir()
		if err != nil {
			t.Fatal(err)
		}
		if cfg != dir {
			t.Skipf("UserConfigDir=%q does not use XDG_CONFIG_HOME=%q", cfg, dir)
		}
	}

	if err := UpsertHost("produkce", "ssh://root@podnikam.eu", "prod"); err != nil {
		t.Fatal(err)
	}
	list, err := LoadHosts()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "produkce" || list[0].Note != "prod" {
		t.Fatalf("list=%+v", list)
	}

	if err := UpsertHost("prod", "ssh://root@podnikam.eu", ""); err != nil {
		t.Fatal(err)
	}
	list, err = LoadHosts()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "prod" || list[0].Note != "prod" {
		t.Fatalf("upsert overwrite=%+v", list)
	}

	if err := RemoveHost("ssh://root@podnikam.eu"); err != nil {
		t.Fatal(err)
	}
	list, err = LoadHosts()
	if err != nil || len(list) != 0 {
		t.Fatalf("after remove list=%+v err=%v", list, err)
	}
}
