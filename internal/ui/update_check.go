package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cooffeeRequired/dockafe/internal/config"
	"github.com/cooffeeRequired/dockafe/internal/update"
)

type updateCheckMsg struct {
	info update.Info
}

type updateApplyMsg struct {
	path string
	err  error
}

func checkUpdateCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return updateCheckMsg{info: update.CheckLatest(ctx, AppVersion, nil)}
	}
}

func applyUpdateCmd(assetURL, expectedSHA256 string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		path, err := update.Apply(ctx, assetURL, expectedSHA256, nil)
		_ = config.Audit("-", "update_apply", expectedSHA256[:min(12, len(expectedSHA256))], err == nil, errString(err))
		return updateApplyMsg{path: path, err: err}
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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
	m.updateSHA256 = info.ExpectedSHA256
	m.updateAvailable = info.Available
}

func (m Model) versionTitle() string {
	return " " + AppName + " " + AppVersion + " "
}

func (m Model) updateBadge() string {
	if !m.updateAvailable || m.updateLatest == "" {
		return ""
	}
	return updateBadgeStyle.Render(" ↑ " + m.updateLatest + " · U ")
}

func (m Model) askUpdate() (tea.Model, tea.Cmd) {
	if m.busy {
		return m, nil
	}
	if !m.updateAvailable {
		m.status = "checking for updates…"
		return m, checkUpdateCmd()
	}
	if m.updateAssetURL == "" {
		m.status = "update " + m.updateLatest + " has no download asset"
		if m.updateURL != "" {
			m.errMsg = "see " + m.updateURL
		}
		return m, nil
	}
	if m.updateSHA256 == "" {
		m.status = "update blocked — release missing dockafe.sha256"
		m.errMsg = "upload dockafe.sha256 next to the binary on GitHub Releases"
		return m, nil
	}
	m.confirm = confirmUpdate
	m.confirmTarget = m.updateLatest
	m.confirmLabel = "Install dockafe " + m.updateLatest + " (SHA256 verified) over current binary?"
	m.mode = ModeConfirm
	return m, nil
}

func (m Model) doUpdate() (tea.Model, tea.Cmd) {
	m.busy = true
	m.status = "downloading " + m.updateLatest + "…"
	m.errMsg = ""
	return m, applyUpdateCmd(m.updateAssetURL, m.updateSHA256)
}
