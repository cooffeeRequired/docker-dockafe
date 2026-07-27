package ui

import (
	"fmt"

	"github.com/cooffeeRequired/dockafe/internal/config"
)

func (m *Model) loadSettings() {
	s, err := config.LoadSettings()
	if err != nil {
		m.settings = config.DefaultSettings()
		return
	}
	m.settings = s
}

func (m Model) canMutate() error {
	c := m.focusedClient()
	if c != nil && c.IsDemo() {
		return fmt.Errorf("demo mode is read-only")
	}
	if c != nil && c.IsRemoteDaemon() && m.settings.IsRemoteReadOnly() {
		return fmt.Errorf("remote host is read-only — unlock in Settings (tab 6)")
	}
	return nil
}

// guardMutate returns false when mutations are blocked (and sets status/err).
func (m Model) guardMutate() (Model, bool) {
	if err := m.canMutate(); err != nil {
		m.errMsg = err.Error()
		m.status = "blocked · read-only"
		return m, false
	}
	return m, true
}

func (m Model) remoteReadOnlyActive() bool {
	c := m.focusedClient()
	return c != nil && c.IsRemoteDaemon() && m.settings.IsRemoteReadOnly()
}

func (m Model) auditHost() string {
	c := m.focusedClient()
	if c == nil {
		return m.dockerHostLabel()
	}
	h := c.Host()
	if h == "" {
		return m.dockerHostLabel()
	}
	return h
}

func (m Model) audit(action, target string, ok bool, err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	_ = config.Audit(m.auditHost(), action, target, ok, msg)
}
