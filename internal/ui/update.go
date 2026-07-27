package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cooffeeRequired/dockafe/internal/config"
	"github.com/cooffeeRequired/dockafe/internal/docker"
)

func (m Model) refresh(withStats bool) tea.Cmd {
	// Caller must bump dataGen and pass via refreshGen for stale protection.
	return m.refreshPaneCmd(0, withStats, m.dataGen)
}

func (m Model) refreshGen(withStats bool, gen uint64) tea.Cmd {
	return m.refreshPaneCmd(0, withStats, gen)
}

func (m Model) enrichStatsCmd(gen uint64, containers []docker.ContainerInfo) tea.Cmd {
	return m.enrichPaneStatsCmd(0, gen, containers)
}

func (m *Model) applyRightDataMsg(msg dataMsg) tea.Cmd {
	if msg.gen != 0 && msg.gen != m.dataGenRight {
		return nil
	}
	m.loadingRight = false
	if msg.sysInfo != "" {
		m.sysInfoRight = msg.sysInfo
	}
	if msg.err != nil {
		m.errRight = msg.err.Error()
		return nil
	}
	m.errRight = ""
	containers := msg.containers
	groups := msg.groups
	if msg.wantStats {
		containers = mergePreservedStats(m.containersRight, containers)
		groups = docker.ComposeGroupsFrom(containers)
	}
	m.containersRight = containers
	m.groupsRight = groups
	m.lastSyncRight = msg.at
	m.recordStatsFromData(dataMsg{containers: containers, groups: groups})
	m.clampMultiCursors()
	if m.mode == ModeMultiHost {
		m.status = fmt.Sprintf("multi · right sync %s", msg.at.Format("15:04:05"))
	}
	if msg.wantStats && !m.statsEnrichBusyRight && hasRunningContainers(containers) {
		m.statsEnrichBusyRight = true
		return m.enrichPaneStatsCmd(1, msg.gen, containers)
	}
	return nil
}

func (m *Model) applyRightStatsEnrich(msg statsEnrichMsg) tea.Cmd {
	m.statsEnrichBusyRight = false
	if msg.gen != 0 && msg.gen != m.dataGenRight {
		return nil
	}
	if msg.err != nil {
		m.errRight = msg.err.Error()
		return nil
	}
	m.containersRight = applyStatsByID(m.containersRight, msg.containers)
	m.groupsRight = docker.ComposeGroupsFrom(m.containersRight)
	m.lastSyncRight = msg.at
	m.recordStatsFromData(dataMsg{containers: m.containersRight, groups: m.groupsRight})
	m.clampMultiCursors()
	if m.mode == ModeMultiHost {
		m.status = fmt.Sprintf("multi · live · %s", msg.at.Format("15:04:05"))
	}
	return nil
}

func (m *Model) startRefresh(withStats bool) tea.Cmd {
	m.dataGen++
	return m.refreshGen(withStats, m.dataGen)
}

func hasRunningContainers(containers []docker.ContainerInfo) bool {
	for _, c := range containers {
		if c.Running {
			return true
		}
	}
	return false
}

func (m Model) runAction(fn func(context.Context) error, okMsg string) tea.Cmd {
	if err := m.canMutate(); err != nil {
		return func() tea.Msg {
			return actionDoneMsg{err: err, msg: okMsg}
		}
	}
	host := m.auditHost()
	target := okMsg
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		err := fn(ctx)
		_ = config.Audit(host, "mutate", target, err == nil, errString(err))
		return actionDoneMsg{err: err, msg: okMsg}
	}
}

func (m Model) fetchLogs() tea.Cmd {
	return m.fetchLogsGen(m.logsGen, m.logsTarget, m.logsName)
}

func (m *Model) startFetchLogs() tea.Cmd {
	m.logsGen++
	return m.fetchLogsGen(m.logsGen, m.logsTarget, m.logsName)
}

func (m Model) fetchLogsGen(gen uint64, id, name string) tea.Cmd {
	client := m.focusedClient()
	return func() tea.Msg {
		if id == "" {
			return logsMsg{gen: gen, targetID: id, err: fmt.Errorf("empty container id")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		body, err := client.ContainerLogs(ctx, id, "500", true)
		if err != nil {
			return logsMsg{gen: gen, targetID: id, err: fmt.Errorf("%s: %w", name, err)}
		}
		return logsMsg{gen: gen, targetID: id, body: body}
	}
}

func (m Model) fetchDetail() tea.Cmd {
	tab := m.tab
	// Capture selection NOW (before async), not later from stale cursor.
	var id, name string
	switch tab {
	case TabContainers:
		id, name = m.currentContainer()
	case TabImages:
		id = m.currentImageID()
	case TabVolumes:
		name = m.currentVolumeName()
	case TabNetworks:
		id, name = m.currentNetwork()
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()

		switch tab {
		case TabCompose:
			return contentMsg{mode: ModeDetail}
		case TabContainers:
			if id == "" {
				return contentMsg{err: fmt.Errorf("no container"), mode: ModeDetail}
			}
			inspect, err := m.focusedClient().InspectContainer(ctx, id)
			if err != nil {
				return contentMsg{err: err, mode: ModeDetail, targetID: id, targetName: name}
			}
			top, _ := m.focusedClient().ContainerTop(ctx, id)
			body := fmt.Sprintf(
				"[ l / f = logs ]  [ e = exec ]  [ t = top ]  [ esc = back ]\n\nName: %s\nID: %s\n\n=== PROCESSES ===\n%s\n\n=== INSPECT ===\n%s\n",
				name, id, emptyDash(top), inspect,
			)
			return contentMsg{title: "Container · " + name, body: body, mode: ModeDetail, targetID: id, targetName: name}
		case TabImages:
			if id == "" {
				return contentMsg{err: fmt.Errorf("no image"), mode: ModeDetail}
			}
			inspect, err := m.focusedClient().InspectImage(ctx, id)
			if err != nil {
				return contentMsg{err: err, mode: ModeDetail}
			}
			return contentMsg{title: "Image · " + id, body: inspect, mode: ModeDetail}
		case TabVolumes:
			if name == "" {
				return contentMsg{err: fmt.Errorf("no volume"), mode: ModeDetail}
			}
			inspect, err := m.focusedClient().InspectVolume(ctx, name)
			if err != nil {
				return contentMsg{err: err, mode: ModeDetail}
			}
			return contentMsg{title: "Volume · " + name, body: inspect, mode: ModeDetail, targetName: name}
		case TabNetworks:
			if id == "" {
				return contentMsg{err: fmt.Errorf("no network"), mode: ModeDetail}
			}
			inspect, err := m.focusedClient().InspectNetwork(ctx, id)
			if err != nil {
				return contentMsg{err: err, mode: ModeDetail}
			}
			return contentMsg{title: "Network · " + name, body: inspect, mode: ModeDetail}
		}
		return contentMsg{mode: ModeDetail}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.relayout()
		m.relayoutWizard()
		return m, nil

	case tickMsg:
		// Graphs use a dedicated 500ms stats poll — skip the heavy full refresh there.
		// Never bump dataGen while a refresh is in flight (loading): otherwise SSH
		// responses arrive stale and the UI stays stuck on "loading…".
		if m.mode == ModeMultiHost && !m.busy && m.confirm == confirmNone {
			cmds = append(cmds, m.tickMultiHostCmds()...)
		} else if (m.mode == ModeList || m.mode == ModeComposeDetail) && !m.busy && !m.loading && m.confirm == confirmNone {
			cmds = append(cmds, m.tickOnePane(0)...)
		}
		cmds = append(cmds, tickCmd())
		return m, tea.Batch(cmds...)

	case graphsTickMsg:
		if m.mode != ModeGraphs {
			return m, nil
		}
		cmds := []tea.Cmd{graphsTickCmd()}
		if !m.graphsSampleBusy {
			m.graphsSampleBusy = true
			cmds = append(cmds, m.sampleGraphsCmd())
		}
		return m, tea.Batch(cmds...)

	case graphsSampleMsg:
		m.graphsSampleBusy = false
		if m.mode != ModeGraphs || msg.key != m.graphsKey {
			return m, nil
		}
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.status = "graphs sample failed"
			return m, nil
		}
		m.applyGraphsSample(msg)
		m.errMsg = ""
		if msg.targetErr != nil {
			m.errMsg = msg.targetErr.Error()
		} else if msg.hostErr != nil && !msg.hostCPUOK {
			m.errMsg = msg.hostErr.Error()
		}
		m.status = fmt.Sprintf(
			"conf: %d upd · poll: %s · sample %s",
			statsHistCap, graphsSampleInterval, msg.at.Format("15:04:05"),
		)
		m.refreshGraphsView()
		return m, nil

	case splashReadyMsg:
		m.splashMinDone = true
		if cmd := m.tryLeaveSplash(); cmd != nil {
			return m, cmd
		}
		return m, nil

	case splashAnimMsg:
		if m.mode == ModeSplash {
			return m, splashAnimCmd()
		}
		return m, nil

	case updateCheckMsg:
		m.applyUpdateCheck(msg)
		if m.updateAvailable {
			m.status = "update " + m.updateLatest + " available · press U"
		} else if m.updateErr == "" && m.updateLatest != "" {
			if m.status == "checking for updates…" {
				m.status = "up to date (" + AppVersion + ")"
			}
		} else if m.updateErr != "" && m.status == "checking for updates…" {
			m.status = "update check failed"
		}
		return m, nil

	case updateApplyMsg:
		m.busy = false
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.status = "update failed"
			return m, nil
		}
		m.updateAvailable = false
		m.status = "updated to " + m.updateLatest + " · restarting…"
		m.errMsg = ""
		return m, tea.Quit

	case eventWatchStartedMsg:
		cmd := m.applyEventWatchStart(msg)
		return m, cmd

	case eventMsg:
		m.appendEvent(msg.ev)
		m.refreshEventsView()
		if m.mode == ModeList && msg.ev.Level == docker.EventCritLevel {
			m.status = "event: " + msg.ev.Message + " · press E"
		}
		return m, m.pollEventCmd()

	case eventErrMsg:
		if msg.err != nil {
			m.eventAlert = "events: " + msg.err.Error()
		}
		return m, nil

	case hostsLoadedMsg:
		if msg.err != nil {
			m.hostErr = msg.err.Error()
		}
		m.hostEndpoints = msg.endpoints
		if m.hostCursor >= len(m.hostEndpoints) {
			m.hostCursor = 0
		}
		return m, nil

	case hostSwitchMsg:
		return m, m.applyHostSwitch(msg)

	case hostSavedMsg:
		return m, m.applyHostSaved(msg)

	case volTransferDoneMsg:
		m.busy = false
		m.volTransferKind = volTransferNone
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.status = "transfer failed"
			return m, nil
		}
		m.errMsg = ""
		m.status = "transfer ok · " + msg.path
		m.volListCache = map[string][]docker.VolumeEntry{}
		if m.mode == ModeVolumeTree {
			return m, m.loadVolChildren("")
		}
		return m, nil

	case logsTickMsg:
		if m.mode == ModeLogs && m.logsFollow && !m.busy && m.logsTarget != "" {
			m.logsGen++
			// Use reloadLogs so comma-joined compose targets fetch per container.
			cmds = append(cmds, m.reloadLogs(), logsTickCmd())
		}
		return m, tea.Batch(cmds...)

	case volTreeMsg:
		if msg.gen != m.volGen || msg.volName != m.volName || m.mode != ModeVolumeTree {
			return m, nil
		}
		m.applyVolTreeMsg(msg)
		return m, nil

	case volFileMsg:
		if msg.gen != m.volGen || msg.volName != m.volName || m.mode != ModeVolumeTree {
			return m, nil
		}
		m.busy = false
		if msg.err != nil {
			m.volErr = msg.err.Error()
			m.status = "read failed"
			m.volFileFocus = false
			return m, nil
		}
		m.volErr = ""
		if m.volPreviewPath != msg.path {
			m.volPreviewLine = 0
		}
		m.volPreviewPath = msg.path
		m.volPreview = msg.content
		m.volLint = msg.lint
		if m.volFileFocus {
			m.status = "file focus · " + msg.path + " · " + m.volAccessMode + " · tab = tree · e = edit"
		} else if msg.lint != "" {
			m.status = "lint · " + msg.path + " · " + m.volAccessMode
		} else {
			m.status = "preview · " + msg.path + " · " + m.volAccessMode
		}
		return m, nil

	case volEditorLaunchMsg:
		return m, m.launchVolEditor(msg)

	case volEditorDoneMsg:
		m.busy = false
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.status = "editor failed"
			return m, nil
		}
		if len(msg.pendingWrite) > 0 {
			m.volPendingPath = msg.path
			m.volPendingData = msg.pendingWrite
			m.confirm = confirmVolWrite
			m.confirmTarget = msg.path
			m.confirmLabel = fmt.Sprintf("Save changes to “%s” (%s)?", msg.path, m.volAccessMode)
			m.volReturnMode = ModeVolumeTree
			m.mode = ModeConfirm
			return m, nil
		}
		m.errMsg = ""
		m.status = "saved · " + msg.path + " · " + m.volAccessMode
		m.invalidateVolCache()
		if m.mode != ModeVolumeTree {
			m.mode = ModeVolumeTree
		}
		return m, m.loadVolFile(msg.path, true)

	case dataMsg:
		if msg.pane == 1 {
			return m, m.applyRightDataMsg(msg)
		}
		if msg.gen != 0 && msg.gen != m.dataGen {
			// Superseded refresh — keep waiting for the current gen.
			return m, nil
		}
		m.loading = false
		if msg.sysInfo != "" {
			m.sysInfo = msg.sysInfo
		}
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.status = "Docker API error"
			if m.mode == ModeSplash {
				m.splashDataReady = true
				if cmd := m.tryLeaveSplash(); cmd != nil {
					return m, cmd
				}
			}
			return m, nil
		}
		m.errMsg = ""
		containers := msg.containers
		groups := msg.groups
		if msg.wantStats {
			// Keep last known CPU/MEM visible while the next enrich runs.
			containers = mergePreservedStats(m.containers, containers)
			groups = docker.ComposeGroupsFrom(containers)
		}
		m.groups = groups
		m.containers = containers
		if msg.images != nil {
			m.images = msg.images
		}
		if msg.volumes != nil {
			m.volumes = msg.volumes
		}
		if msg.networks != nil {
			m.networks = msg.networks
		}
		m.lastSync = msg.at
		m.recordStatsFromData(dataMsg{containers: containers, groups: groups})
		m.status = fmt.Sprintf("sync %s · sort %s", msg.at.Format("15:04:05"), m.sortLabel())
		if m.mode == ModeSplash {
			m.splashDataReady = true
			if cmd := m.tryLeaveSplash(); cmd != nil {
				return m, cmd
			}
			return m, nil
		}
		if m.mode == ModeList {
			m.applyRows()
		}
		if m.mode == ModeComposeDetail {
			prefer := ""
			if svc, ok := m.selectedComposeService(); ok {
				prefer = svc.ID
			}
			m.syncComposeServices(prefer)
		}
		if m.mode == ModeGraphs {
			m.status = fmt.Sprintf("graphs · sync %s · esc back", msg.at.Format("15:04:05"))
			m.refreshGraphsView()
		}
		m.clampMultiCursors()

		var cmds []tea.Cmd
		if msg.wantStats && !m.statsEnrichBusy && hasRunningContainers(containers) {
			m.statsEnrichBusy = true
			had := false
			for _, c := range containers {
				if containerHasStats(c) {
					had = true
					break
				}
			}
			if !had {
				m.status = fmt.Sprintf("sync %s · loading CPU/MEM…", msg.at.Format("15:04:05"))
			} else {
				m.status = fmt.Sprintf("sync %s · live CPU/MEM", msg.at.Format("15:04:05"))
			}
			cmds = append(cmds, m.enrichPaneStatsCmd(0, msg.gen, containers))
		}
		return m, tea.Batch(cmds...)

	case statsEnrichMsg:
		if msg.pane == 1 {
			return m, m.applyRightStatsEnrich(msg)
		}
		m.statsEnrichBusy = false
		if msg.gen != 0 && msg.gen != m.dataGen {
			return m, nil
		}
		if msg.err != nil {
			m.status = "CPU/MEM enrich failed"
			return m, nil
		}
		m.containers = applyStatsByID(m.containers, msg.containers)
		m.groups = docker.ComposeGroupsFrom(m.containers)
		m.lastSync = msg.at
		m.recordStatsFromData(dataMsg{
			containers: m.containers,
			groups:     m.groups,
		})
		m.status = fmt.Sprintf("sync %s · live · sort %s", msg.at.Format("15:04:05"), m.sortLabel())
		if m.mode == ModeList {
			m.applyRows()
		}
		if m.mode == ModeComposeDetail {
			prefer := ""
			if svc, ok := m.selectedComposeService(); ok {
				prefer = svc.ID
			}
			m.syncComposeServices(prefer)
		}
		if m.mode == ModeGraphs {
			m.refreshGraphsView()
		}
		m.clampMultiCursors()
		return m, nil

	case actionDoneMsg:
		m.busy = false
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.status = "action failed"
		} else {
			m.errMsg = ""
			m.status = msg.msg
			if m.mode == ModeCreateCompose || m.mode == ModePullImage {
				m.mode = ModeList
			}
		}
		if m.composeProject != "" && m.mode == ModeList && m.returnToCompose {
			m.mode = ModeComposeDetail
		}
		m.dataGen++
		return m, m.refreshGen(true, m.dataGen)

	case contentMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.status = "error: " + truncate(msg.err.Error(), 60)
			if m.composeProject != "" {
				m.mode = ModeComposeDetail
			} else {
				m.mode = ModeList
			}
			return m, nil
		}
		if msg.targetID != "" {
			m.targetID = msg.targetID
			m.targetName = msg.targetName
		}
		if m.tab == TabCompose && msg.body == "" && msg.title == "" {
			m.openComposeDetailLocal()
		} else {
			m.detailTitle = msg.title
			m.detailBody = msg.body
			m.mode = ModeDetail
			m.vp.SetContent(msg.body)
			m.vp.GotoTop()
			m.status = "detail · press l = logs"
		}
		m.relayout()
		return m, nil

	case logsMsg:
		if msg.gen != 0 && msg.gen != m.logsGen {
			return m, nil
		}
		if msg.targetID != "" && m.logsTarget != "" && msg.targetID != m.logsTarget {
			return m, nil
		}
		m.mode = ModeLogs
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.status = "logs failed"
			m.vp.SetContent("Error loading logs:\n\n" + msg.err.Error() + "\n\nPress r to retry, esc back.")
			m.relayout()
			return m, nil
		}
		m.errMsg = ""
		atBottom := m.vp.AtBottom()
		body := msg.body
		if !msg.colored {
			body = colorizeLogs(msg.body)
		}
		m.detailBody = body
		m.vp.SetContent(body)
		m.relayout()
		// Re-run active search on refreshed logs
		if m.logsSearchQuery != "" && (m.logsSearchOpen || len(m.logsSearchMatches) > 0) {
			m.logsSearchInput.SetValue(m.logsSearchQuery)
			m.applyLogSearch()
		} else if m.logsFollow || atBottom {
			m.vp.GotoBottom()
		}
		m.status = fmt.Sprintf("logs %s · follow=%v · / find · ctrl+g regex", m.logsName, m.logsFollow)
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	if m.mode == ModeFilter {
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		m.applyRows()
		return m, cmd
	}
	if m.mode == ModeDetail || m.mode == ModeLogs || m.mode == ModeHelp || m.mode == ModeGraphs || m.mode == ModeEvents {
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.mode == ModeSplash {
		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "U":
			return m.askUpdate()
		case "enter", " ", "esc":
			if m.splashDataReady {
				m.splashMinDone = true
				_ = m.tryLeaveSplash()
			}
			return m, nil
		}
		return m, nil
	}

	if m.mode == ModeVolumeTree {
		return m.handleVolumeTreeKey(msg)
	}

	if m.mode == ModeHosts {
		return m.handleHostsKey(msg)
	}

	if m.mode == ModeMultiHost {
		return m.handleMultiHostKey(msg)
	}

	if m.mode == ModeVolTransfer {
		return m.handleVolTransferKey(msg)
	}

	if m.mode == ModeConfirm {
		return m.handleConfirm(msg)
	}

	if m.mode == ModeFilter {
		switch key {
		case "esc":
			m.mode = ModeList
			m.filter.Blur()
			m.applyRows()
			return m, nil
		case "enter":
			m.mode = ModeList
			m.filter.Blur()
			m.applyRows()
			m.status = "filter: " + m.filter.Value()
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		}
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		m.applyRows()
		return m, cmd
	}

	if m.mode == ModeCreateCompose {
		return m.handleComposeWizardKey(msg)
	}
	if m.mode == ModePullImage {
		return m.handleImageWizardKey(msg)
	}

	if m.mode == ModeHelp {
		if key == "esc" || key == "?" || key == "q" {
			m.backFromPanel()
			m.relayout()
			return m, nil
		}
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}

	if m.mode == ModeComposeDetail {
		return m.handleComposeDetailKey(msg)
	}

	if m.mode == ModeList && m.tab == TabSettings {
		switch key {
		case "up", "k", "down", "j", "enter", " ":
			return m.handleSettingsKey(msg)
		}
	}

	if m.mode == ModeDetail || m.mode == ModeLogs || m.mode == ModeGraphs || m.mode == ModeEvents {
		// Search input takes over when open
		if m.mode == ModeLogs && m.logsSearchOpen && m.logsSearchInput.Focused() {
			return m.handleLogSearchKey(msg)
		}

		switch key {
		case "esc":
			if m.mode == ModeLogs && (m.logsSearchOpen || len(m.logsSearchMatches) > 0) {
				m.closeLogSearch()
				return m, nil
			}
			m.backFromPanel()
			m.relayout()
			return m, nil
		case "q":
			m.backFromPanel()
			m.relayout()
			return m, nil
		case "backspace":
			if m.mode == ModeLogs && (m.logsSearchOpen || len(m.logsSearchMatches) > 0) {
				m.closeLogSearch()
				return m, nil
			}
			if m.mode == ModeEvents || m.mode == ModeGraphs {
				m.backFromPanel()
				m.relayout()
				return m, nil
			}
			m.backFromPanel()
			m.relayout()
			return m, nil
		case "/", "ctrl+f", "alt+f":
			if m.mode == ModeLogs {
				m.openLogSearch(false)
				return m, nil
			}
		case "ctrl+g", "ctrl+shift+f", "alt+shift+f":
			if m.mode == ModeLogs {
				m.openLogSearch(true)
				return m, nil
			}
		case "n":
			if m.mode == ModeLogs && len(m.logsSearchMatches) > 0 {
				m.nextSearchMatch(false)
				return m, nil
			}
		case "N":
			if m.mode == ModeLogs && len(m.logsSearchMatches) > 0 {
				m.nextSearchMatch(true)
				return m, nil
			}
		case "ctrl+up", "pgup":
			m.vp.PageUp()
			if m.mode == ModeLogs {
				m.logsFollow = false
				m.status = "follow=false · page up"
			}
			return m, nil
		case "ctrl+down", "pgdown":
			m.vp.PageDown()
			if m.mode == ModeLogs && m.vp.AtBottom() {
				m.logsFollow = true
				m.status = "follow=true · page down"
			}
			return m, nil
		case "ctrl+u":
			m.vp.HalfPageUp()
			if m.mode == ModeLogs {
				m.logsFollow = false
			}
			return m, nil
		case "ctrl+d":
			m.vp.HalfPageDown()
			return m, nil
		case "f", "l":
			if m.mode == ModeEvents || m.mode == ModeGraphs {
				return m, nil
			}
			// Volume detail: f opens file tree (no container logs).
			if m.mode == ModeDetail && key == "f" && strings.HasPrefix(m.detailTitle, "Volume · ") {
				return m.openVolumeTree()
			}
			// In DETAIL: f and l open logs. In LOGS: f toggles follow, l refreshes.
			if m.mode == ModeLogs && key == "f" {
				m.logsFollow = !m.logsFollow
				m.status = fmt.Sprintf("follow=%v", m.logsFollow)
				if m.logsFollow {
					m.vp.GotoBottom()
					m.logsGen++
					return m, tea.Batch(m.reloadLogs(), logsTickCmd())
				}
				return m, nil
			}
			if m.mode == ModeLogs && key == "l" {
				m.logsGen++
				return m, m.reloadLogs()
			}
			return m.openLogsForTarget()
		case "F":
			// reserved — list filter only

		case "r":
			if m.mode == ModeGraphs {
				if m.graphsSampleBusy {
					m.status = "sample already in flight…"
					return m, nil
				}
				m.graphsSampleBusy = true
				m.status = "sampling…"
				return m, m.sampleGraphsCmd()
			}
			if m.mode == ModeEvents {
				m.refreshEventsView()
				m.status = "events view refreshed"
				return m, nil
			}
			if m.mode == ModeLogs {
				m.logsGen++
				return m, m.reloadLogs()
			}
			if m.targetID != "" {
				return m, m.fetchTargetInspect()
			}
			return m, m.fetchDetail()
		case "e":
			if m.mode == ModeEvents || m.mode == ModeGraphs {
				return m, nil
			}
			return m.execTarget()
		case "t":
			if m.mode == ModeEvents || m.mode == ModeGraphs {
				return m, nil
			}
			return m.openTopForTarget()
		case "s":
			return m.actionOnTarget("start")
		case "x":
			return m.actionOnTarget("stop")
		case "R":
			return m.actionOnTarget("restart")
		case "g":
			m.vp.GotoTop()
			if m.mode == ModeLogs {
				m.logsFollow = false
			}
			return m, nil
		case "G":
			m.vp.GotoBottom()
			if m.mode == ModeLogs {
				m.logsFollow = true
			}
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			if m.mode == ModeLogs && m.logsSearchOpen {
				m.applyLogSearch()
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}

	// ModeList
	if m.busy {
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	}

	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.openHelp()
		return m, nil
	case "/":
		if m.tab == TabSettings {
			return m, nil
		}
		m.mode = ModeFilter
		m.filter.Focus()
		return m, nil
	case "ctrl+u":
		m.filter.SetValue("")
		m.applyRows()
		m.status = "filter cleared"
		return m, nil
	case "o":
		m.cycleSort()
		m.applyRows()
		m.status = "sort " + m.sortLabel()
		return m, nil
	case "O":
		m.sortAsc = !m.sortAsc
		m.applyRows()
		m.status = "sort " + m.sortLabel()
		return m, nil
	case "F":
		m.runningOnly = !m.runningOnly
		m.applyRows()
		if m.tab == TabVolumes {
			m.status = fmt.Sprintf("in-use-only=%v", m.runningOnly)
		} else {
			m.status = fmt.Sprintf("running-only=%v", m.runningOnly)
		}
		return m, nil
	case "f":
		if m.tab == TabVolumes {
			return m.openVolumeTree()
		}
	case "tab", "]":
		m.tab = Tab((int(m.tab) + 1) % len(tabNames))
		m.ensureSortForTab()
		m.relayout()
		return m, nil
	case "shift+tab", "[":
		m.tab = Tab((int(m.tab) - 1 + len(tabNames)) % len(tabNames))
		m.ensureSortForTab()
		m.relayout()
		return m, nil
	case "1", "2", "3", "4", "5", "6":
		m.tab = Tab(int(key[0] - '1'))
		m.ensureSortForTab()
		m.relayout()
		return m, nil
	case "r":
		m.loading = true
		m.status = "refreshing…"
		return m, m.refresh(true)
	case "U":
		return m.askUpdate()
	case "s":
		return m.startSelected()
	case "x":
		return m.stopSelected()
	case "R":
		return m.restartSelected()
	case "b":
		return m.askRebuild()
	case "d":
		return m.askRemove()
	case "D":
		return m.askRemoveAll()
	case "p":
		return m.pauseSelected()
	case "k":
		return m.askKill()
	case "l":
		return m.openLogsForTarget()
	case "g":
		return m.openGraphs()
	case "E":
		return m.openEvents()
	case "H":
		m.hostPickTarget = hostPickLeft
		m.returnToMulti = false
		return m.openHosts()
	case "M":
		return m.openMultiHost()
	case "t":
		return m.openTopForTarget()
	case "e":
		return m.execTarget()
	case "i":
		return m.openDetail()
	case "P":
		return m.askPrune()
	case "n":
		if m.tab == TabCompose {
			m.openComposeWizard()
			m.relayoutWizard()
			return m, nil
		}
		if m.tab == TabImages {
			m.openImageWizard()
			m.relayoutWizard()
			return m, nil
		}
		m.status = "n = new compose (Compose) or image (Images)"
		return m, nil
	case "N":
		m.openImageWizard()
		m.relayoutWizard()
		return m, nil
	case "c":
		if m.tab == TabCompose {
			name := m.currentComposeName()
			if name != "" {
				m.selectedGroup = name
				m.tab = TabContainers
				m.ensureSortForTab()
				m.relayout()
				m.status = "project filter: " + name
			}
		}
		return m, nil
	case "enter":
		return m.openDetail()
	case "esc":
		m.selectedGroup = ""
		m.errMsg = ""
		m.filter.SetValue("")
		m.applyRows()
		m.status = "filters cleared"
		return m, nil
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *Model) ensureSortForTab() {
	ok := false
	for _, k := range m.sortKeysForTab() {
		if k == m.sortKey {
			ok = true
			break
		}
	}
	if !ok {
		m.sortKey = m.sortKeysForTab()[0]
	}
}

func (m Model) handleConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	restore := func() {
		if m.volReturnMode == ModeVolumeTree {
			m.mode = ModeVolumeTree
			m.volReturnMode = 0
			return
		}
		if m.composeProject != "" {
			m.mode = ModeComposeDetail
		} else {
			m.mode = ModeList
		}
	}
	switch msg.String() {
	case "y", "Y", "enter":
		action := m.confirm
		target := m.confirmTarget
		m.confirm = confirmNone
		m.confirmTarget = ""
		if action == confirmVolWrite {
			if m2, ok := m.guardMutate(); !ok {
				m.volPendingPath = ""
				m.volPendingData = nil
				return m2, nil
			}
			path := m.volPendingPath
			data := m.volPendingData
			m.volPendingPath = ""
			m.volPendingData = nil
			m.mode = ModeVolumeTree
			m.volReturnMode = 0
			m.busy = true
			m.status = "saving " + path + "…"
			vol := m.volName
			client := m.focusedClient()
			host := m.auditHost()
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()
				err := client.WriteVolumeFile(ctx, vol, path, data)
				errMsg := ""
				if err != nil {
					errMsg = err.Error()
				}
				_ = config.Audit(host, "volume_write", vol+"/"+path, err == nil, errMsg)
				if err != nil {
					return volEditorDoneMsg{path: path, err: err}
				}
				return volEditorDoneMsg{path: path}
			}
		}
		restore()
		switch action {
		case confirmRemove:
			return m.doRemove(target)
		case confirmRemoveAll:
			return m.doRemoveAll(target)
		case confirmRebuild:
			return m.doRebuild(target)
		case confirmKill:
			return m.doKill(target)
		case confirmPrune:
			return m.doPrune(target)
		case confirmUpdate:
			return m.doUpdate()
		}
	case "n", "N", "esc":
		m.confirm = confirmNone
		m.confirmTarget = ""
		m.volPendingPath = ""
		m.volPendingData = nil
		restore()
		m.status = "cancelled"
	}
	return m, nil
}

func (m *Model) openHelp() {
	m.mode = ModeHelp
	m.detailTitle = "Help"
	m.detailBody = helpTextFull()
	m.vp.SetContent(m.detailBody)
	m.vp.GotoTop()
	m.relayout()
}

func (m Model) openDetail() (tea.Model, tea.Cmd) {
	if m.tab == TabCompose {
		m.openComposeDetailLocal()
		m.relayout()
		return m, nil
	}
	if m.tab == TabContainers {
		id, name := m.currentContainer()
		if id == "" {
			m.status = "no container"
			return m, nil
		}
		m.targetID = id
		m.targetName = name
	}
	m.status = "loading detail…"
	return m, m.fetchDetail()
}

func (m Model) openLogs() (tea.Model, tea.Cmd) {
	if m.tab == TabCompose {
		name := m.currentComposeName()
		ids := []string{}
		names := []string{}
		for _, g := range m.groups {
			if g.Name != name {
				continue
			}
			for _, c := range g.Containers {
				ids = append(ids, c.ID)
				names = append(names, c.Name)
			}
		}
		if len(ids) == 0 {
			m.status = "no containers in project"
			return m, nil
		}
		m.logsTarget = strings.Join(ids, ",")
		m.logsName = name
		m.mode = ModeLogs
		m.logsFollow = true
		m.logsGen++
		m.detailTitle = "Logs · compose " + name
		m.vp.SetContent("loading logs…")
		m.relayout()
		return m, tea.Batch(m.fetchComposeLogsGen(m.logsGen, ids, names), logsTickCmd())
	}
	if m.tab == TabContainers {
		id, name := m.currentContainer()
		if id == "" {
			return m, nil
		}
		m.logsTarget = id
		m.logsName = name
		m.mode = ModeLogs
		m.logsFollow = true
		m.logsGen++
		m.detailTitle = "Logs · " + name
		m.vp.SetContent("loading logs…")
		m.relayout()
		return m, tea.Batch(m.fetchLogsGen(m.logsGen, id, name), logsTickCmd())
	}
	m.status = "logs only on Compose/Containers"
	return m, nil
}

func (m Model) fetchComposeLogs(ids, names []string) tea.Cmd {
	return m.fetchComposeLogsGen(m.logsGen, ids, names)
}

func (m Model) fetchComposeLogsGen(gen uint64, ids, names []string) tea.Cmd {
	client := m.focusedClient()
	targetKey := strings.Join(ids, ",")
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		bodies := make([]string, len(ids))
		errs := make([]error, len(ids))
		labels := make([]string, len(ids))
		for i, id := range ids {
			if i < len(names) && names[i] != "" {
				labels[i] = names[i]
			} else {
				labels[i] = short(id)
			}
			body, err := client.ContainerLogs(ctx, id, "150", true)
			bodies[i] = body
			errs[i] = err
		}
		merged := mergeComposeLogs(labels, bodies, errs)
		return logsMsg{gen: gen, targetID: targetKey, body: merged, colored: true}
	}
}

func (m Model) openTop() (tea.Model, tea.Cmd) {
	if m.tab != TabContainers {
		m.status = "top only on Containers"
		return m, nil
	}
	id, name := m.currentContainer()
	if id == "" {
		return m, nil
	}
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		body, err := m.focusedClient().ContainerTop(ctx, id)
		if err != nil {
			return contentMsg{err: err, mode: ModeDetail}
		}
		return contentMsg{title: "Top · " + name, body: body, mode: ModeDetail}
	}
}
