package docker

import (
	"strings"
)

// ParseHealth extracts Docker health from a list Status string
// (e.g. "Up 2 hours (healthy)").
func ParseHealth(status string) string {
	s := strings.ToLower(status)
	switch {
	case strings.Contains(s, "(healthy)"):
		return "healthy"
	case strings.Contains(s, "(unhealthy)"):
		return "unhealthy"
	case strings.Contains(s, "(health: starting)"), strings.Contains(s, "(health:starting)"):
		return "starting"
	default:
		return ""
	}
}

// ParseOOMHint detects OOM from Status text or exit code 137 (128+SIGKILL).
func ParseOOMHint(status string) bool {
	s := strings.ToLower(status)
	if strings.Contains(s, "oom") {
		return true
	}
	return strings.Contains(s, "(137)")
}

// StateLabel returns a short UI label combining state, health, and OOM.
func StateLabel(state, health string, oom bool) string {
	if oom {
		return "oom"
	}
	switch health {
	case "healthy":
		return "healthy"
	case "unhealthy":
		return "unhealthy"
	case "starting":
		return "starting"
	}
	if state == "" {
		return "-"
	}
	return state
}
