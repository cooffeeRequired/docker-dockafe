package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cooffeeRequired/dockafe/internal/config"
	"github.com/cooffeeRequired/dockafe/internal/docker"
)

type composeWizardStep int

const (
	cwMeta composeWizardStep = iota
	cwServices
	cwYAML
)

type composeField int

const (
	cfProject composeField = iota
	cfDir
	cfSvcName
	cfSvcImage
	cfSvcPorts
	cfSvcEnv
	cfSvcVolumes
	cfSvcRestart
	cfCount
)

type imageWizardField int

const (
	iwImage imageWizardField = iota
	iwMode
	iwName
	iwPorts
	iwCount
)

type ImageAddMode int

const (
	ImageModePermanent ImageAddMode = iota // docker pull — stays local
	ImageModeTemporary                     // pull + run --rm
)

func (m ImageAddMode) String() string {
	if m == ImageModeTemporary {
		return "temporary (pull + run --rm)"
	}
	return "permanent (docker pull)"
}

func (m *Model) initWizards() {
	mk := func(placeholder string) textinput.Model {
		ti := textinput.New()
		ti.Placeholder = placeholder
		ti.CharLimit = 256
		ti.Width = 48
		return ti
	}

	m.cwInputs = make([]textinput.Model, cfCount)
	m.cwInputs[cfProject] = mk("my-app")
	m.cwInputs[cfDir] = mk("~/docker-projects/my-app")
	m.cwInputs[cfSvcName] = mk("web")
	m.cwInputs[cfSvcImage] = mk("nginx:alpine")
	m.cwInputs[cfSvcPorts] = mk("8080:80")
	m.cwInputs[cfSvcEnv] = mk("KEY=value,OTHER=1")
	m.cwInputs[cfSvcVolumes] = mk("./data:/data")
	m.cwInputs[cfSvcRestart] = mk("unless-stopped")
	m.cwInputs[cfSvcRestart].SetValue("unless-stopped")

	ta := textarea.New()
	ta.Placeholder = "compose.yaml"
	ta.CharLimit = 100000
	ta.ShowLineNumbers = true
	ta.SetWidth(80)
	ta.SetHeight(16)
	m.cwYAML = ta

	m.iwInputs = make([]textinput.Model, iwCount)
	m.iwInputs[iwImage] = mk("nginx:alpine / postgres:16")
	m.iwInputs[iwMode] = mk("permanent | temporary")
	m.iwInputs[iwMode].SetValue("permanent")
	m.iwInputs[iwName] = mk("optional container name")
	m.iwInputs[iwPorts] = mk("8080:80")

	m.cwServices = nil
	m.cwStep = cwMeta
	m.cwFocus = cfProject
	m.iwFocus = iwImage
	m.iwMode = ImageModePermanent
	m.acIndex = 0
	m.acItems = nil
}

func (m *Model) openComposeWizard() {
	m.initWizards()
	home, _ := os.UserHomeDir()
	m.cwInputs[cfDir].SetValue(filepath.Join(home, "docker-projects"))
	m.mode = ModeCreateCompose
	m.cwStep = cwMeta
	m.focusComposeField(cfProject)
	m.status = "new compose · Tab fields · Enter next · ? help"
	m.loadImageSuggestions()
}

func (m *Model) openImageWizard() {
	m.initWizards()
	m.mode = ModePullImage
	m.iwFocus = iwImage
	m.iwInputs[iwImage].Focus()
	m.status = "add image · Tab fields · p/t permanent/temp · Enter run"
	m.loadImageSuggestions()
}

func (m *Model) loadImageSuggestions() {
	tags := make([]string, 0, len(m.images)*2)
	for _, img := range m.images {
		for _, part := range strings.Split(img.Tags, ", ") {
			part = strings.TrimSpace(part)
			if part == "" || part == "<none>" {
				continue
			}
			tags = append(tags, part)
		}
	}
	// common defaults
	defaults := []string{
		"nginx:alpine", "nginx:latest",
		"postgres:16-alpine", "postgres:15-alpine",
		"redis:alpine", "redis:7-alpine",
		"mysql:8", "mariadb:11",
		"node:22-alpine", "node:20-alpine",
		"python:3.12-slim", "golang:1.22-alpine",
		"adminer:latest", "traefik:v3.0",
		"busybox:latest", "alpine:latest",
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(tags)+len(defaults))
	for _, t := range append(tags, defaults...) {
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	m.imageSuggestions = out
}

func (m *Model) focusComposeField(f composeField) {
	for i := range m.cwInputs {
		m.cwInputs[i].Blur()
	}
	m.cwYAML.Blur()
	m.cwFocus = f
	if m.cwStep == cwYAML {
		m.cwYAML.Focus()
		return
	}
	if int(f) < len(m.cwInputs) {
		m.cwInputs[f].Focus()
	}
	m.refreshAutocomplete()
}

func (m *Model) refreshAutocomplete() {
	m.acItems = nil
	m.acIndex = 0
	var q string
	switch {
	case m.mode == ModeCreateCompose && m.cwFocus == cfSvcImage:
		q = m.cwInputs[cfSvcImage].Value()
	case m.mode == ModePullImage && m.iwFocus == iwImage:
		q = m.iwInputs[iwImage].Value()
	default:
		return
	}
	q = strings.ToLower(strings.TrimSpace(q))
	for _, s := range m.imageSuggestions {
		if q == "" || strings.Contains(strings.ToLower(s), q) {
			m.acItems = append(m.acItems, s)
			if len(m.acItems) >= 8 {
				break
			}
		}
	}
}

func (m Model) handleComposeWizardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		if m.cwStep == cwYAML {
			m.cwStep = cwServices
			m.focusComposeField(cfSvcName)
			m.status = "back to form"
			return m, nil
		}
		m.mode = ModeList
		m.status = "compose wizard cancelled"
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "ctrl+y":
		if m.cwStep != cwYAML {
			m.syncYAMLFromSpec()
			m.cwStep = cwYAML
			m.cwYAML.Focus()
			for i := range m.cwInputs {
				m.cwInputs[i].Blur()
			}
			m.status = "YAML · ctrl+s save · ctrl+u up · esc back"
			m.relayoutWizard()
			return m, nil
		}
		m.cwStep = cwServices
		m.focusComposeField(cfSvcName)
		return m, nil
	case "ctrl+s":
		return m.saveCompose(false)
	case "ctrl+u":
		return m.saveCompose(true)
	case "ctrl+a":
		if m.cwStep != cwYAML {
			return m.addComposeService()
		}
	case "ctrl+w":
		if m.cwStep != cwYAML && len(m.cwServices) > 0 {
			removed := m.cwServices[len(m.cwServices)-1].Name
			m.cwServices = m.cwServices[:len(m.cwServices)-1]
			m.status = fmt.Sprintf("removed %s · %d remaining", removed, len(m.cwServices))
			return m, nil
		}
	case "tab":
		if m.cwStep == cwYAML {
			return m, nil
		}
		next := composeField((int(m.cwFocus) + 1) % int(cfCount))
		m.focusComposeField(next)
		if next >= cfSvcName {
			m.cwStep = cwServices
		}
		return m, nil
	case "shift+tab":
		if m.cwStep == cwYAML {
			return m, nil
		}
		prev := composeField((int(m.cwFocus) - 1 + int(cfCount)) % int(cfCount))
		m.focusComposeField(prev)
		return m, nil
	case "down":
		if m.cwStep != cwYAML && m.cwFocus == cfSvcImage && len(m.acItems) > 0 {
			m.acIndex = (m.acIndex + 1) % len(m.acItems)
			return m, nil
		}
		if m.cwStep != cwYAML {
			next := composeField((int(m.cwFocus) + 1) % int(cfCount))
			m.focusComposeField(next)
			return m, nil
		}
	case "up":
		if m.cwStep != cwYAML && m.cwFocus == cfSvcImage && len(m.acItems) > 0 {
			m.acIndex = (m.acIndex - 1 + len(m.acItems)) % len(m.acItems)
			return m, nil
		}
		if m.cwStep != cwYAML {
			prev := composeField((int(m.cwFocus) - 1 + int(cfCount)) % int(cfCount))
			m.focusComposeField(prev)
			return m, nil
		}
	case "enter":
		if m.cwStep == cwYAML {
			return m.saveCompose(false)
		}
		if m.cwFocus == cfSvcImage && len(m.acItems) > 0 {
			m.cwInputs[cfSvcImage].SetValue(m.acItems[m.acIndex])
			m.acItems = nil
			m.focusComposeField(cfSvcPorts)
			return m, nil
		}
		if m.cwFocus == cfDir {
			m.cwStep = cwServices
			m.focusComposeField(cfSvcName)
			return m, nil
		}
		if m.cwFocus == cfSvcRestart || m.cwFocus == cfSvcVolumes {
			return m.addComposeService()
		}
		next := composeField((int(m.cwFocus) + 1) % int(cfCount))
		m.focusComposeField(next)
		return m, nil
	}

	if m.cwStep == cwYAML {
		var cmd tea.Cmd
		m.cwYAML, cmd = m.cwYAML.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.cwInputs[m.cwFocus], cmd = m.cwInputs[m.cwFocus].Update(msg)
	if m.cwFocus == cfSvcImage {
		m.refreshAutocomplete()
	}
	if m.cwFocus == cfProject {
		name := strings.TrimSpace(m.cwInputs[cfProject].Value())
		if name != "" {
			home, _ := os.UserHomeDir()
			base := filepath.Join(home, "docker-projects")
			cur := m.cwInputs[cfDir].Value()
			if cur == "" || strings.HasPrefix(cur, base) {
				m.cwInputs[cfDir].SetValue(filepath.Join(base, name))
			}
		}
	}
	return m, cmd
}

func (m Model) addComposeService() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.cwInputs[cfSvcName].Value())
	image := strings.TrimSpace(m.cwInputs[cfSvcImage].Value())
	if name == "" || image == "" {
		m.status = "fill in service name + image"
		return m, nil
	}
	svc := docker.ComposeServiceSpec{
		Name:    name,
		Image:   image,
		Ports:   splitCSV(m.cwInputs[cfSvcPorts].Value()),
		Env:     splitCSV(m.cwInputs[cfSvcEnv].Value()),
		Volumes: splitCSV(m.cwInputs[cfSvcVolumes].Value()),
		Restart: strings.TrimSpace(m.cwInputs[cfSvcRestart].Value()),
	}
	m.cwServices = append(m.cwServices, svc)
	m.cwInputs[cfSvcName].SetValue("")
	m.cwInputs[cfSvcImage].SetValue("")
	m.cwInputs[cfSvcPorts].SetValue("")
	m.cwInputs[cfSvcEnv].SetValue("")
	m.cwInputs[cfSvcVolumes].SetValue("")
	m.cwInputs[cfSvcRestart].SetValue("unless-stopped")
	m.focusComposeField(cfSvcName)
	m.status = fmt.Sprintf("added: %s (%s) · total %d · ctrl+y YAML · ctrl+s save", name, image, len(m.cwServices))
	return m, nil
}

func (m *Model) syncYAMLFromSpec() {
	spec := m.composeSpec()
	m.cwYAML.SetValue(spec.RenderYAML())
}

func (m Model) composeSpec() docker.ComposeProjectSpec {
	name := strings.TrimSpace(m.cwInputs[cfProject].Value())
	if name == "" {
		name = "app"
	}
	dir := strings.TrimSpace(m.cwInputs[cfDir].Value())
	if strings.HasPrefix(dir, "~/") {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, dir[2:])
	}
	return docker.ComposeProjectSpec{
		Name:     name,
		Dir:      dir,
		Services: append([]docker.ComposeServiceSpec{}, m.cwServices...),
	}
}

func (m Model) saveCompose(up bool) (tea.Model, tea.Cmd) {
	if m2, ok := m.guardMutate(); !ok {
		return m2, nil
	}
	spec := m.composeSpec()
	if len(spec.Services) == 0 && m.cwStep != cwYAML {
		m.status = "add at least one service (a) or edit YAML (y)"
		return m, nil
	}
	yamlBody := ""
	if m.cwStep == cwYAML || strings.TrimSpace(m.cwYAML.Value()) != "" {
		yamlBody = m.cwYAML.Value()
		if yamlBody == "" {
			m.syncYAMLFromSpec()
			yamlBody = m.cwYAML.Value()
		}
	}
	m.busy = true
	m.status = "saving compose…"
	host := m.auditHost()
	return m, func() tea.Msg {
		path, err := m.client.WriteComposeFile(spec, yamlBody)
		if err != nil {
			_ = config.Audit(host, "compose_save", spec.Name, false, err.Error())
			return actionDoneMsg{err: err}
		}
		if !up {
			_ = config.Audit(host, "compose_save", path, true, "")
			return actionDoneMsg{msg: "saved: " + path}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		dir := filepath.Dir(path)
		if err := m.client.ComposeConfigCheck(ctx, dir); err != nil {
			_ = config.Audit(host, "compose_up", path, false, err.Error())
			return actionDoneMsg{err: fmt.Errorf("invalid yaml: %w", err), msg: path}
		}
		msg, err := m.client.ComposeUp(ctx, dir, spec.Name)
		if err != nil {
			_ = config.Audit(host, "compose_up", path, false, err.Error())
			return actionDoneMsg{err: err, msg: "file: " + path}
		}
		_ = config.Audit(host, "compose_up", path, true, "")
		return actionDoneMsg{msg: "saved+up: " + path + " · " + truncate(msg, 80)}
	}
}

func (m Model) handleImageWizardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.mode = ModeList
		m.status = "image wizard cancelled"
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "p":
		m.iwMode = ImageModePermanent
		m.iwInputs[iwMode].SetValue("permanent")
		m.status = "mode: permanent pull"
		return m, nil
	case "t":
		m.iwMode = ImageModeTemporary
		m.iwInputs[iwMode].SetValue("temporary")
		m.status = "mode: temporary run --rm"
		return m, nil
	case "tab", "down":
		if key == "down" && m.iwFocus == iwImage && len(m.acItems) > 0 {
			m.acIndex = (m.acIndex + 1) % len(m.acItems)
			return m, nil
		}
		m.iwInputs[m.iwFocus].Blur()
		m.iwFocus = imageWizardField((int(m.iwFocus) + 1) % int(iwCount))
		m.iwInputs[m.iwFocus].Focus()
		m.refreshAutocomplete()
		return m, nil
	case "shift+tab", "up":
		if key == "up" && m.iwFocus == iwImage && len(m.acItems) > 0 {
			if m.acIndex > 0 {
				m.acIndex--
			} else {
				m.acIndex = len(m.acItems) - 1
			}
			return m, nil
		}
		m.iwInputs[m.iwFocus].Blur()
		m.iwFocus = imageWizardField((int(m.iwFocus) - 1 + int(iwCount)) % int(iwCount))
		m.iwInputs[m.iwFocus].Focus()
		m.refreshAutocomplete()
		return m, nil
	case "enter":
		if m.iwFocus == iwImage && len(m.acItems) > 0 && m.iwInputs[iwImage].Value() != m.acItems[m.acIndex] {
			// if typed prefix matches suggestions, accept selected
			if strings.TrimSpace(m.iwInputs[iwImage].Value()) == "" || suggestionPrefixMatch(m.iwInputs[iwImage].Value(), m.acItems[m.acIndex]) {
				m.iwInputs[iwImage].SetValue(m.acItems[m.acIndex])
				m.acItems = nil
				m.iwInputs[iwImage].Blur()
				m.iwFocus = iwMode
				m.iwInputs[iwMode].Focus()
				return m, nil
			}
		}
		return m.runImageWizard()
	}

	var cmd tea.Cmd
	m.iwInputs[m.iwFocus], cmd = m.iwInputs[m.iwFocus].Update(msg)
	if m.iwFocus == iwImage {
		m.refreshAutocomplete()
	}
	modeVal := strings.ToLower(m.iwInputs[iwMode].Value())
	if strings.HasPrefix(modeVal, "t") {
		m.iwMode = ImageModeTemporary
	} else {
		m.iwMode = ImageModePermanent
	}
	return m, cmd
}

func suggestionPrefixMatch(typed, suggestion string) bool {
	t := strings.ToLower(strings.TrimSpace(typed))
	s := strings.ToLower(suggestion)
	return strings.HasPrefix(s, t) || strings.Contains(s, t)
}

func (m Model) runImageWizard() (tea.Model, tea.Cmd) {
	if m2, ok := m.guardMutate(); !ok {
		return m2, nil
	}
	ref := strings.TrimSpace(m.iwInputs[iwImage].Value())
	if ref == "" && len(m.acItems) > 0 {
		ref = m.acItems[m.acIndex]
		m.iwInputs[iwImage].SetValue(ref)
	}
	if ref == "" {
		m.status = "enter image (autocomplete ↑↓)"
		return m, nil
	}
	name := strings.TrimSpace(m.iwInputs[iwName].Value())
	ports := splitCSV(m.iwInputs[iwPorts].Value())
	mode := m.iwMode
	m.busy = true
	m.status = "pulling/starting " + ref + "…"
	host := m.auditHost()
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		var msg string
		var err error
		action := "pull_image"
		if mode == ImageModeTemporary {
			action = "run_temporary"
			msg, err = m.client.RunTemporaryContainer(ctx, ref, name, ports)
		} else {
			msg, err = m.client.PullImage(ctx, ref)
		}
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		_ = config.Audit(host, action, ref, err == nil, errMsg)
		return actionDoneMsg{err: err, msg: msg}
	}
}

func (m *Model) relayoutWizard() {
	if m.width < 40 {
		return
	}
	// Wizard widgets exist only after openComposeWizard / openImageWizard.
	if len(m.cwInputs) == 0 && len(m.iwInputs) == 0 {
		return
	}
	if len(m.cwInputs) > 0 {
		m.cwYAML.SetWidth(max(40, m.width-8))
		m.cwYAML.SetHeight(max(8, m.height-12))
		for i := range m.cwInputs {
			m.cwInputs[i].Width = max(20, m.width-24)
		}
	}
	for i := range m.iwInputs {
		m.iwInputs[i].Width = max(20, m.width-24)
	}
}

func splitCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (m Model) viewComposeWizard() string {
	title := lipgloss.JoinHorizontal(lipgloss.Top,
		titleStyle.Render(" "+AppName+" "+AppVersion+" "),
		"  ",
		activeTabStyle.Render(" NEW COMPOSE "),
	)

	var b strings.Builder
	b.WriteString(metaStyle.Render("Create docker compose project"))
	b.WriteString("\n\n")

	if m.cwStep == cwYAML {
		b.WriteString(metaStyle.Render("YAML (ctrl+y = back · ctrl+s save · ctrl+u up -d)"))
		b.WriteString("\n")
		b.WriteString(m.cwYAML.View())
	} else {
		labels := []string{
			"Project name", "Directory", "Service name", "Image (autocomplete)",
			"Ports", "Env KEY=val", "Volumes", "Restart",
		}
		for i, label := range labels {
			cursor := "  "
			style := metaStyle
			if composeField(i) == m.cwFocus {
				cursor = "▸ "
				style = statusStyle
			}
			b.WriteString(style.Render(cursor + label + ":"))
			b.WriteString("\n")
			b.WriteString("  " + m.cwInputs[i].View() + "\n")
			if composeField(i) == cfSvcImage {
				b.WriteString(m.renderAutocomplete())
			}
		}
		b.WriteString("\n")
		b.WriteString(metaStyle.Render(fmt.Sprintf("Services (%d):", len(m.cwServices))))
		b.WriteString("\n")
		if len(m.cwServices) == 0 {
			b.WriteString(helpStyle.Render("  (none yet — fill in service + ctrl+a / Enter)"))
			b.WriteString("\n")
		}
		for _, s := range m.cwServices {
			b.WriteString(fmt.Sprintf("  • %s ← %s", s.Name, s.Image))
			if len(s.Ports) > 0 {
				b.WriteString("  ports=" + strings.Join(s.Ports, ","))
			}
			b.WriteString("\n")
		}
	}

	body := panelStyle.Width(max(40, m.width-2)).Render(b.String())
	help := helpFooterWizard(true)
	status := statusStyle.Render(m.status)
	if m.errMsg != "" {
		status = errorStyle.Render(m.errMsg)
	}
	return lipgloss.JoinVertical(lipgloss.Left, title, body, status, help)
}

func (m Model) viewImageWizard() string {
	title := lipgloss.JoinHorizontal(lipgloss.Top,
		titleStyle.Render(" "+AppName+" "+AppVersion+" "),
		"  ",
		activeTabStyle.Render(" ADD IMAGE "),
	)

	var b strings.Builder
	b.WriteString(metaStyle.Render("Add Docker image"))
	b.WriteString("\n\n")

	labels := []string{"Image", "Mode (permanent/temporary)", "Container name (temp)", "Ports (temp)"}
	for i, label := range labels {
		cursor := "  "
		style := metaStyle
		if imageWizardField(i) == m.iwFocus {
			cursor = "▸ "
			style = statusStyle
		}
		b.WriteString(style.Render(cursor + label + ":"))
		b.WriteString("\n  " + m.iwInputs[i].View() + "\n")
		if imageWizardField(i) == iwImage {
			b.WriteString(m.renderAutocomplete())
		}
	}
	b.WriteString("\n")
	b.WriteString(metaStyle.Render("Current mode: " + m.iwMode.String()))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("permanent = docker pull (stays locally)\ntemporary = pull + docker run -d --rm (container removed on stop)"))

	body := panelStyle.Width(max(40, m.width-2)).Render(b.String())
	help := helpFooterWizard(false)
	status := statusStyle.Render(m.status)
	if m.errMsg != "" {
		status = errorStyle.Render(m.errMsg)
	}
	return lipgloss.JoinVertical(lipgloss.Left, title, body, status, help)
}

func (m Model) renderAutocomplete() string {
	if len(m.acItems) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(helpStyle.Render("  autocomplete:"))
	b.WriteString("\n")
	for i, item := range m.acItems {
		if i == m.acIndex {
			b.WriteString(composeSelStyle.Render("    → " + item))
		} else {
			b.WriteString(helpStyle.Render("      " + item))
		}
		b.WriteString("\n")
	}
	return b.String()
}
