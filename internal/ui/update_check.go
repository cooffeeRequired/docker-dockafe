package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cooffeeRequired/dockafe/internal/update"
)

type updateCheckMsg struct {
	info update.Info
}

func checkUpdateCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return updateCheckMsg{info: update.CheckLatest(ctx, AppVersion, nil)}
	}
}

func (m *Model) applyUpdateCheck(msg updateCheckMsg) {
	info := msg.info
	if info.CheckError != "" {
		m.updateErr = info.CheckError
		return
	}
	m.updateErr = ""
	m.updateLatest = info.Latest
	m.updateURL = info.URL
	m.updateAssetURL = info.AssetURL
	m.updateAvailable = info.Available
}

func (m Model) versionTitle() string {
	return " " + AppName + " " + AppVersion + " "
}

func (m Model) updateBadge() string {
	if !m.updateAvailable || m.updateLatest == "" {
		return ""
	}
	return updateBadgeStyle.Render("UPDATE " + m.updateLatest + " · U ")
}
