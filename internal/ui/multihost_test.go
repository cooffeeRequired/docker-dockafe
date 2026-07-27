package ui

import (
	"testing"

	"github.com/cooffeeRequired/dockafe/internal/docker"
)

func TestFilterComposeGroupsRunningOnly(t *testing.T) {
	m := Model{
		runningOnly: true,
		groups: []docker.ComposeGroup{
			{Name: "up", Running: 1, Total: 1},
			{Name: "down", Running: 0, Total: 2},
		},
	}
	got := m.filterComposeGroups(m.groups)
	if len(got) != 1 || got[0].Name != "up" {
		t.Fatalf("got=%+v", got)
	}
}

func TestMultiCursorClamp(t *testing.T) {
	m := Model{
		tab: TabCompose,
		groups: []docker.ComposeGroup{
			{Name: "a", Running: 1, Total: 1},
		},
		groupsRight: []docker.ComposeGroup{
			{Name: "b", Running: 1, Total: 1},
			{Name: "c", Running: 1, Total: 1},
		},
		multiCursorL: 9,
		multiCursorR: 9,
	}
	m.clampMultiCursors()
	if m.multiCursorL != 0 {
		t.Fatalf("left cursor=%d", m.multiCursorL)
	}
	if m.multiCursorR != 1 {
		t.Fatalf("right cursor=%d", m.multiCursorR)
	}
}

func TestFocusedClientUsesActionPane(t *testing.T) {
	left := docker.NewDemo()
	right := docker.NewDemo()
	m := Model{client: left, clientRight: right, mode: ModeGraphs, actionPane: 1}
	if m.focusedClient() != right {
		t.Fatal("expected right client for actionPane=1")
	}
	m.actionPane = 0
	if m.focusedClient() != left {
		t.Fatal("expected left client")
	}
}
