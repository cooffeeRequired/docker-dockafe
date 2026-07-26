package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/coffee/docker-tui/internal/docker"
)

type volNode struct {
	entry    docker.VolumeEntry
	depth    int
	expanded bool
	loaded   bool
	children []*volNode
}

type volTreeMsg struct {
	gen     uint64
	volName string
	entries []docker.VolumeEntry
	path    string // "" = root load; else children of path
	err     error
}

type volFileMsg struct {
	gen     uint64
	volName string
	path    string
	content string
	err     error
	lint    string
}

type volEditorLaunchMsg struct {
	rel      string
	editPath string
	tmpFile  bool
}

type volEditorDoneMsg struct {
	path         string
	err          error
	pendingWrite []byte // non-nil → ask confirm before WriteVolumeFile
}

func (m *Model) invalidateVolCache() {
	m.volListCache = map[string][]docker.VolumeEntry{}
}

func (m *Model) openVolumeTree() (tea.Model, tea.Cmd) {
	name := m.volumeNameFromContext()
	if name == "" {
		m.status = "no volume selected"
		return *m, nil
	}
	m.mode = ModeVolumeTree
	m.volName = name
	m.volCursor = 0
	m.volOffset = 0
	m.volPreviewPath = ""
	m.volPreview = ""
	m.volLint = ""
	m.volErr = ""
	m.volFileFocus = false
	m.volPreviewLine = 0
	m.invalidateVolCache()
	m.volRoot = &volNode{
		entry: docker.VolumeEntry{Name: name, Path: "", IsDir: true},
		depth: -1,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	m.volAccessMode = m.client.VolumeAccessMode(ctx, name)
	cancel()
	m.status = "loading volume tree… · " + m.volAccessMode
	m.busy = true
	return *m, m.loadVolChildren("")
}

func (m Model) volumeNameFromContext() string {
	if m.tab == TabVolumes {
		if n := m.currentVolumeName(); n != "" {
			return n
		}
	}
	if strings.HasPrefix(m.detailTitle, "Volume · ") {
		return strings.TrimPrefix(m.detailTitle, "Volume · ")
	}
	if m.targetName != "" && m.tab == TabVolumes {
		return m.targetName
	}
	return ""
}

func (m *Model) loadVolChildren(rel string) tea.Cmd {
	m.volGen++
	gen := m.volGen
	client := m.client
	vol := m.volName
	if cached, ok := m.volListCache[rel]; ok {
		entries := cached
		return func() tea.Msg {
			return volTreeMsg{gen: gen, volName: vol, entries: entries, path: rel}
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		entries, err := client.ListVolumeDir(ctx, vol, rel)
		return volTreeMsg{gen: gen, volName: vol, entries: entries, path: rel, err: err}
	}
}

func (m *Model) loadVolFile(rel string, withLint bool) tea.Cmd {
	m.volGen++
	gen := m.volGen
	client := m.client
	vol := m.volName
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		data, err := client.ReadVolumeFile(ctx, vol, rel)
		if err != nil {
			return volFileMsg{gen: gen, volName: vol, path: rel, err: err}
		}
		if !utf8.Valid(data) || looksBinary(data) {
			return volFileMsg{
				gen:     gen,
				volName: vol,
				path:    rel,
				content: fmt.Sprintf("⟪ binary file · %d bytes · open with e in external editor ⟫", len(data)),
			}
		}
		raw := string(data)
		truncated := false
		if len(raw) > 256*1024 {
			raw = raw[:256*1024]
			truncated = true
		}
		content := highlightSource(rel, raw)
		if truncated {
			content += "\n\n" + helpDescStyle.Render("… truncated (256 KiB preview)")
		}
		lint := ""
		if withLint {
			lint = lintSource(rel, data)
		}
		return volFileMsg{gen: gen, volName: vol, path: rel, content: content, lint: lint}
	}
}

func looksBinary(data []byte) bool {
	n := len(data)
	if n > 8000 {
		n = 8000
	}
	for i := 0; i < n; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

func (m *Model) applyVolTreeMsg(msg volTreeMsg) {
	m.busy = false
	if msg.err != nil {
		m.volErr = msg.err.Error()
		m.status = "volume tree failed"
		return
	}
	m.volErr = ""
	if m.volListCache == nil {
		m.volListCache = map[string][]docker.VolumeEntry{}
	}
	m.volListCache[msg.path] = msg.entries
	parent := m.findVolNode(msg.path)
	if parent == nil {
		parent = m.volRoot
	}
	if parent == nil {
		m.volErr = "volume tree not initialized"
		return
	}
	parent.loaded = true
	parent.expanded = true
	parent.children = make([]*volNode, 0, len(msg.entries))
	for _, e := range msg.entries {
		parent.children = append(parent.children, &volNode{
			entry: e,
			depth: parent.depth + 1,
		})
	}
	m.status = fmt.Sprintf("volume %s · %d entries · %s", m.volName, len(msg.entries), m.volAccessMode)
}

func (m *Model) findVolNode(rel string) *volNode {
	if rel == "" {
		return m.volRoot
	}
	var walk func(*volNode) *volNode
	walk = func(n *volNode) *volNode {
		if n.entry.Path == rel {
			return n
		}
		for _, ch := range n.children {
			if found := walk(ch); found != nil {
				return found
			}
		}
		return nil
	}
	return walk(m.volRoot)
}

func (m Model) flatVolNodes() []*volNode {
	var out []*volNode
	var walk func(*volNode)
	walk = func(n *volNode) {
		if n != m.volRoot {
			out = append(out, n)
		}
		if n.expanded {
			for _, ch := range n.children {
				walk(ch)
			}
		}
	}
	if m.volRoot != nil {
		walk(m.volRoot)
	}
	return out
}

func (m Model) selectedVolNode() *volNode {
	nodes := m.flatVolNodes()
	if m.volCursor < 0 || m.volCursor >= len(nodes) {
		return nil
	}
	return nodes[m.volCursor]
}

func (m Model) viewVolumeTree() string {
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		titleStyle.Render(" "+AppName+" "),
		"  ",
		activeTabStyle.Render(" VOLUME FILES "),
		"  ",
		metaStyle.Render(m.volName),
	)
	if m.volFileFocus {
		header = lipgloss.JoinHorizontal(lipgloss.Top, header, "  ",
			activeTabStyle.Render(" FILE "),
		)
	}

	nodes := m.flatVolNodes()
	treeH := max(5, m.height-6)
	offset := m.volOffset
	if m.volCursor < offset {
		offset = m.volCursor
	}
	if m.volCursor >= offset+treeH {
		offset = m.volCursor - treeH + 1
	}

	var lines []string
	end := offset + treeH
	if end > len(nodes) {
		end = len(nodes)
	}
	if len(nodes) == 0 {
		lines = append(lines, helpDescStyle.Render("  (empty…)"))
	}
	for i := offset; i < end; i++ {
		n := nodes[i]
		pad := strings.Repeat("  ", max(0, n.depth))
		icon := fileIcon(n.entry)
		branch := "+-"
		if n.entry.IsDir {
			if n.expanded {
				branch = "[-]"
			} else {
				branch = "[+]"
			}
		}
		name := n.entry.Name
		if m.volFileFocus {
			// Compact tree: hide icons / sizes
			label := fmt.Sprintf("%s%s %s", pad, branch, name)
			style := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
			if i == m.volCursor {
				style = lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true)
			}
			lines = append(lines, style.Render(ansi.Truncate(label, max(12, m.width*18/100-4), "…")))
			continue
		}
		label := fmt.Sprintf("%s%s %s %s", pad, branch, icon, name)
		if n.entry.IsDir && n.loaded {
			label += fmt.Sprintf("  %s", helpDescStyle.Render(fmt.Sprintf("(%d)", len(n.children))))
		} else if !n.entry.IsDir && n.entry.Size > 0 {
			label += "  " + helpDescStyle.Render(humanSize(n.entry.Size))
		}
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		if i == m.volCursor {
			style = lipgloss.NewStyle().
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("33")).
				Bold(true)
			label = style.Render(" " + label + " ")
		} else {
			label = style.Render(" " + label)
		}
		lines = append(lines, label)
	}

	treeBody := strings.Join(lines, "\n")
	previewTitle := m.volPreviewPath
	if previewTitle == "" {
		previewTitle = "preview"
	}
	if m.volFileFocus && m.volPreviewPath != "" {
		total := strings.Count(m.volPreview, "\n") + 1
		previewTitle = fmt.Sprintf("%s  ·  L%d/%d  ·  tab=tree", m.volPreviewPath, m.volPreviewLine+1, total)
	}

	preview := m.volPreview
	if preview == "" {
		preview = helpDescStyle.Render("Enter file · Tab focus file · e edit (LSP) · L lint")
	}
	preview += formatLintPanel(m.volLint)

	var leftW, rightW int
	if m.volFileFocus {
		leftW = max(14, m.width*16/100)
		rightW = max(40, m.width-leftW-4)
	} else {
		leftW = max(24, m.width*42/100)
		rightW = max(20, m.width-leftW-6)
	}

	treeBorder := lipgloss.Color("240")
	fileBorder := lipgloss.Color("240")
	if m.volFileFocus {
		fileBorder = lipgloss.Color("33")
		treeBorder = lipgloss.Color("236")
	} else {
		treeBorder = lipgloss.Color("33")
	}

	left := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(treeBorder).
		Width(leftW).
		Height(treeH + 1).
		Render(treeBody)

	previewBody := scrollANSILines(preview, m.volPreviewLine, treeH-2, rightW-4)
	previewInner := helpKeyStyle.Render(ansi.Truncate(previewTitle, rightW-4, "…")) + "\n\n" + previewBody
	right := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(fileBorder).
		Width(rightW).
		Render(previewInner)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)

	status := statusStyle.Render(m.status)
	if m.volErr != "" {
		status = errorStyle.Render("ERR: " + truncate(m.volErr, m.width-10))
	} else if m.errMsg != "" {
		status = errorStyle.Render("ERR: " + truncate(m.errMsg, m.width-10))
	}

	var help string
	if m.confirm == confirmVolWrite {
		help = confirmStyle.Render(m.confirmLabel + "  [y/n]")
	} else if m.volFileFocus {
		help = renderHelpRow("file", []helpBinding{
			{"↑↓", "scroll"},
			{"pgup/pgdn", "page"},
			{"g/G", "top/end"},
			{"e", "edit+LSP"},
			{"L", "lint"},
			{"o", "open dir"},
			{"tab", "tree"},
			{"esc", "tree"},
		})
	} else {
		help = renderHelpRow("files", []helpBinding{
			{"↑↓", "move"},
			{"Enter", "open"},
			{"tab", "focus file"},
			{"e", "edit+LSP"},
			{"L", "lint"},
			{"o", "open dir"},
			{"r", "reload"},
			{"esc", "back"},
		})
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, status, help)
}

func fileIcon(e docker.VolumeEntry) string {
	if e.IsDir {
		return "dir"
	}
	ext := strings.ToLower(path.Ext(e.Name))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "py"
	case ".rs":
		return "rs"
	case ".ts", ".tsx", ".js", ".jsx":
		return "js"
	case ".json", ".yaml", ".yml", ".toml":
		return "cfg"
	case ".md", ".txt", ".log":
		return "txt"
	default:
		return "file"
	}
}

func humanSize(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%dB", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1fK", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1fM", float64(n)/(1024*1024))
	}
}

func scrollANSILines(s string, startLine, maxLines, width int) string {
	lines := strings.Split(s, "\n")
	if startLine < 0 {
		startLine = 0
	}
	if startLine > len(lines) {
		startLine = len(lines)
	}
	end := startLine + maxLines
	if end > len(lines) {
		end = len(lines)
	}
	if startLine >= end {
		return ""
	}
	chunk := lines[startLine:end]
	for i, line := range chunk {
		chunk[i] = ansi.Truncate(line, width, "…")
	}
	return strings.Join(chunk, "\n")
}

func (m Model) previewLineCount() int {
	if m.volPreview == "" {
		return 0
	}
	return strings.Count(m.volPreview, "\n") + 1
}

func (m *Model) clampPreviewScroll(viewLines int) {
	total := m.previewLineCount()
	maxStart := max(0, total-viewLines)
	if m.volPreviewLine < 0 {
		m.volPreviewLine = 0
	}
	if m.volPreviewLine > maxStart {
		m.volPreviewLine = maxStart
	}
}

func (m Model) handleVolumeTreeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	nodes := m.flatVolNodes()
	viewLines := max(5, m.height-8)

	// File-pane focus: scroll / edit / tab back
	if m.volFileFocus {
		switch key {
		case "tab":
			m.volFileFocus = false
			m.status = "tree focus"
			return m, nil
		case "esc", "backspace":
			m.volFileFocus = false
			m.status = "tree focus"
			return m, nil
		case "q":
			m.volFileFocus = false
			m.mode = ModeDetail
			m.relayout()
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			m.volPreviewLine--
			m.clampPreviewScroll(viewLines)
			return m, nil
		case "down", "j":
			m.volPreviewLine++
			m.clampPreviewScroll(viewLines)
			return m, nil
		case "pgup", "ctrl+up":
			m.volPreviewLine -= viewLines
			m.clampPreviewScroll(viewLines)
			return m, nil
		case "pgdown", "ctrl+down":
			m.volPreviewLine += viewLines
			m.clampPreviewScroll(viewLines)
			return m, nil
		case "g":
			m.volPreviewLine = 0
			return m, nil
		case "G":
			m.volPreviewLine = 1 << 30
			m.clampPreviewScroll(viewLines)
			return m, nil
		case "e":
			if m.volPreviewPath != "" {
				return m, m.openVolInEditor(m.volPreviewPath)
			}
			n := m.selectedVolNode()
			if n != nil && !n.entry.IsDir {
				return m, m.openVolInEditor(n.entry.Path)
			}
			m.status = "no file open"
			return m, nil
		case "L":
			path := m.volPreviewPath
			if path == "" {
				if n := m.selectedVolNode(); n != nil && !n.entry.IsDir {
					path = n.entry.Path
				}
			}
			if path == "" {
				m.status = "no file open"
				return m, nil
			}
			m.status = "linting " + path + "…"
			return m, m.loadVolFile(path, true)
		case "r":
			if m.volPreviewPath != "" {
				return m, m.loadVolFile(m.volPreviewPath, true)
			}
		}
		return m, nil
	}

	switch key {
	case "esc", "q", "backspace":
		m.mode = ModeDetail
		m.relayout()
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "tab":
		n := m.selectedVolNode()
		if m.volPreviewPath != "" {
			m.volFileFocus = true
			m.volPreviewLine = 0
			m.status = "file focus · tab returns to tree · e = LSP editor"
			return m, nil
		}
		if n == nil || n.entry.IsDir {
			m.status = "select a file, then Tab"
			return m, nil
		}
		m.volFileFocus = true
		m.volPreviewLine = 0
		m.status = "opening file…"
		return m, m.loadVolFile(n.entry.Path, true)
	case "up", "k":
		if m.volCursor > 0 {
			m.volCursor--
			if m.volCursor < m.volOffset {
				m.volOffset = m.volCursor
			}
		}
		return m, nil
	case "down", "j":
		if m.volCursor < len(nodes)-1 {
			m.volCursor++
			treeH := max(5, m.height-6)
			if m.volCursor >= m.volOffset+treeH {
				m.volOffset = m.volCursor - treeH + 1
			}
		}
		return m, nil
	case "g":
		m.volCursor = 0
		m.volOffset = 0
		return m, nil
	case "G":
		if len(nodes) > 0 {
			m.volCursor = len(nodes) - 1
			treeH := max(5, m.height-6)
			m.volOffset = max(0, m.volCursor-treeH+1)
		}
		return m, nil
	case "r":
		m.busy = true
		m.status = "reloading…"
		m.invalidateVolCache()
		m.volRoot.children = nil
		m.volRoot.loaded = false
		m.volCursor = 0
		m.volFileFocus = false
		return m, m.loadVolChildren("")
	case "o":
		return m.openVolInFileManager()
	case " ", "enter":
		n := m.selectedVolNode()
		if n == nil {
			return m, nil
		}
		if n.entry.IsDir {
			if n.expanded {
				n.expanded = false
				return m, nil
			}
			if !n.loaded {
				m.busy = true
				m.status = "loading " + n.entry.Path + "…"
				return m, m.loadVolChildren(n.entry.Path)
			}
			n.expanded = true
			return m, nil
		}
		m.volPreviewLine = 0
		m.status = "reading " + n.entry.Path + "…"
		return m, m.loadVolFile(n.entry.Path, true)
	case "e":
		n := m.selectedVolNode()
		if n == nil || n.entry.IsDir {
			m.status = "select a file to edit"
			return m, nil
		}
		return m, m.openVolInEditor(n.entry.Path)
	case "L":
		n := m.selectedVolNode()
		if n == nil || n.entry.IsDir {
			m.status = "select a file to lint"
			return m, nil
		}
		m.status = "linting " + n.entry.Path + "…"
		return m, m.loadVolFile(n.entry.Path, true)
	}
	return m, nil
}

func (m Model) openVolInEditor(rel string) tea.Cmd {
	client := m.client
	vol := m.volName
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		if host, ok := client.HostPathForVolumeFile(ctx, vol, rel); ok {
			return volEditorLaunchMsg{rel: rel, editPath: host, tmpFile: false}
		}
		data, err := client.ReadVolumeFile(ctx, vol, rel)
		if err != nil {
			return volEditorDoneMsg{path: rel, err: err}
		}
		tmp, err := os.CreateTemp("", "dockafe-vol-*"+filepath.Ext(rel))
		if err != nil {
			return volEditorDoneMsg{path: rel, err: err}
		}
		if _, err := tmp.Write(data); err != nil {
			tmp.Close()
			_ = os.Remove(tmp.Name())
			return volEditorDoneMsg{path: rel, err: err}
		}
		_ = tmp.Close()
		return volEditorLaunchMsg{rel: rel, editPath: tmp.Name(), tmpFile: true}
	}
}

func (m Model) openVolInFileManager() (tea.Model, tea.Cmd) {
	n := m.selectedVolNode()
	rel := ""
	if m.volPreviewPath != "" {
		rel = m.volPreviewPath
	} else if n != nil {
		rel = n.entry.Path
		if n.entry.IsDir {
			rel = n.entry.Path
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	host, ok := m.client.HostPathForVolumeFile(ctx, m.volName, rel)
	if !ok {
		// try volume root
		if mp, ok2 := m.client.VolumeHostAccessible(ctx, m.volName); ok2 {
			host = mp
			if rel != "" {
				host = filepath.Join(mp, filepath.FromSlash(rel))
			}
			ok = true
		}
	}
	if !ok {
		m.status = "open in file manager only on local host mount"
		return m, nil
	}
	dir := host
	if fi, err := os.Stat(host); err == nil && !fi.IsDir() {
		dir = filepath.Dir(host)
	}
	bin, err := exec.LookPath("xdg-open")
	if err != nil {
		m.status = "xdg-open not found"
		return m, nil
	}
	c := exec.Command(bin, dir)
	m.status = "opened " + dir
	return m, tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return volEditorDoneMsg{path: rel, err: err}
		}
		return volEditorDoneMsg{path: rel}
	})
}

func (m Model) launchVolEditor(msg volEditorLaunchMsg) tea.Cmd {
	cmd, err := resolveEditorCommand(msg.editPath)
	if err != nil {
		if msg.tmpFile {
			_ = os.Remove(msg.editPath)
		}
		return func() tea.Msg {
			return volEditorDoneMsg{path: msg.rel, err: err}
		}
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if msg.tmpFile {
			raw, readErr := os.ReadFile(msg.editPath)
			_ = os.Remove(msg.editPath)
			if readErr != nil {
				if err == nil {
					err = readErr
				}
				return volEditorDoneMsg{path: msg.rel, err: err}
			}
			// Require confirm before writing back through Docker/helper
			return volEditorDoneMsg{path: msg.rel, err: err, pendingWrite: raw}
		}
		return volEditorDoneMsg{path: msg.rel, err: err}
	})
}

func resolveEditorCommand(file string) (*exec.Cmd, error) {
	for _, cand := range []string{
		os.Getenv("DOCKAFE_EDITOR"),
		os.Getenv("EDITOR"),
		os.Getenv("VISUAL"),
		"hx", "helix", "nvim", "vim", "code", "codium", "nano",
	} {
		cand = strings.TrimSpace(cand)
		if cand == "" {
			continue
		}
		fields := strings.Fields(cand)
		if len(fields) == 0 {
			continue
		}
		bin := fields[0]
		path, err := exec.LookPath(bin)
		if err != nil {
			continue
		}
		args := append([]string{}, fields[1:]...)
		base := filepath.Base(path)
		if (base == "code" || base == "codium" || base == "code-insiders") && !containsStr(args, "--wait") {
			args = append(args, "--wait")
		}
		args = append(args, file)
		return exec.Command(path, args...), nil
	}
	return nil, fmt.Errorf("no editor found (install helix/nvim or set $EDITOR / DOCKAFE_EDITOR)")
}

func containsStr(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func pickLSPEditor() string {
	cmd, err := resolveEditorCommand("/dev/null")
	if err != nil {
		return ""
	}
	return cmd.Path
}
