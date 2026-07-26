package docker

import "testing"

func TestParseHealth(t *testing.T) {
	cases := map[string]string{
		"Up 2 hours (healthy)":            "healthy",
		"Up About a minute (unhealthy)":   "unhealthy",
		"Up 5 seconds (health: starting)": "starting",
		"Up 2 hours":                      "",
		"Exited (0) 2 days ago":           "",
	}
	for in, want := range cases {
		if got := ParseHealth(in); got != want {
			t.Fatalf("ParseHealth(%q)=%q want %q", in, got, want)
		}
	}
}

func TestParseOOMHint(t *testing.T) {
	if !ParseOOMHint("Exited (137) 10 seconds ago") {
		t.Fatal("137 should hint OOM")
	}
	if !ParseOOMHint("Exited (1) OOMKilled") {
		t.Fatal("oom text")
	}
	if ParseOOMHint("Exited (0) 2 days ago") {
		t.Fatal("clean exit")
	}
}

func TestStateLabel(t *testing.T) {
	if StateLabel("running", "healthy", false) != "healthy" {
		t.Fatal("healthy")
	}
	if StateLabel("exited", "", true) != "oom" {
		t.Fatal("oom")
	}
	if StateLabel("running", "", false) != "running" {
		t.Fatal("running")
	}
}
