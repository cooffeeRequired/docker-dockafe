package ui

import (
	"testing"

	"github.com/cooffeeRequired/dockafe/internal/docker"
)

func TestMergePreservedStats(t *testing.T) {
	prev := []docker.ContainerInfo{
		{ID: "a", CPU: "1.5%", Mem: "10MiB", CPUVal: 1.5, MemBytes: 10_000_000},
	}
	next := []docker.ContainerInfo{
		{ID: "a", CPU: "-", Mem: "-", Name: "x"},
		{ID: "b", CPU: "-", Mem: "-"},
	}
	got := mergePreservedStats(prev, next)
	if got[0].CPU != "1.5%" || got[0].MemBytes != 10_000_000 {
		t.Fatalf("preserved=%+v", got[0])
	}
	if got[0].Name != "x" {
		t.Fatalf("inventory fields should update, name=%q", got[0].Name)
	}
	if containerHasStats(got[1]) {
		t.Fatalf("new id should stay empty: %+v", got[1])
	}
}

func TestApplyStatsByID(t *testing.T) {
	current := []docker.ContainerInfo{
		{ID: "a", Name: "keep", CPU: "1.0%", CPUVal: 1, MemBytes: 1},
	}
	sampled := []docker.ContainerInfo{
		{ID: "a", CPU: "9.0%", Mem: "2MiB", CPUVal: 9, MemBytes: 2_000_000},
	}
	got := applyStatsByID(current, sampled)
	if got[0].Name != "keep" || got[0].CPUVal != 9 || got[0].MemBytes != 2_000_000 {
		t.Fatalf("%+v", got[0])
	}
}
