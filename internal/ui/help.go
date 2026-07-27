package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	helpKeyStyle = lipgloss.NewStyle().
			Foreground(cHelpKey).
			Bold(true)
	helpDescStyle = lipgloss.NewStyle().
			Foreground(cMuted)
	helpTagStyle = lipgloss.NewStyle().
			Foreground(cFaint).
			Bold(true)
	helpSepStyle = lipgloss.NewStyle().
			Foreground(cBorderDim)
	helpTitleStyle = lipgloss.NewStyle().
			Foreground(cHelpKey).
			Bold(true)
	helpSectionStyle = lipgloss.NewStyle().
				Foreground(cHelpSec).
				Bold(true)
)

// helpBinding formats a single "key desc" pair for the footer.
type helpBinding struct {
	Key  string
	Desc string
}

const helpSep = "  "

func renderBindings(items []helpBinding) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts,
			helpKeyStyle.Render(item.Key)+" "+helpDescStyle.Render(item.Desc),
		)
	}
	return strings.Join(parts, helpSepStyle.Render(helpSep))
}

func renderHelpRow(tag string, items []helpBinding) string {
	return helpTagStyle.Render(tag) + " " + renderBindings(items)
}

func helpFooterList(tab Tab) string {
	filterLabel := "running"
	if tab == TabVolumes {
		filterLabel = "in-use"
	}
	nav := renderHelpRow("nav", []helpBinding{
		{"?", "help"},
		{"/", "filter"},
		{"o/O", "sort"},
		{"F", filterLabel},
		{"1-6/[ ]", "tabs"},
		{"r", "refresh"},
		{"U", "update"},
		{"E", "events"},
		{"H", "hosts"},
		{"M", "multi"},
		{"q", "quit"},
	})

	if tab == TabSettings {
		return nav + "\n" + renderHelpRow("set", []helpBinding{
			{"↑↓", "move"},
			{"Enter", "open"},
			{"H", "hosts"},
			{"M", "multi"},
			{"U", "update"},
		})
	}

	var act []helpBinding
	switch tab {
	case TabCompose:
		act = []helpBinding{
			{"n", "new"},
			{"Enter", "open"},
			{"c", "containers"},
			{"s", "start"},
			{"x", "stop"},
			{"R", "restart"},
			{"b", "rebuild"},
			{"l", "logs"},
			{"g", "graphs"},
			{"d/D", "remove"},
		}
	case TabContainers:
		act = []helpBinding{
			{"Enter", "inspect"},
			{"s", "start"},
			{"x", "stop"},
			{"R", "restart"},
			{"p", "pause"},
			{"k", "kill"},
			{"l", "logs"},
			{"g", "graphs"},
			{"t", "top"},
			{"e", "exec"},
			{"d", "remove"},
			{"P", "prune"},
		}
	case TabImages:
		act = []helpBinding{
			{"n/N", "add image"},
			{"Enter", "inspect"},
			{"d", "remove"},
			{"P", "prune"},
		}
	case TabVolumes:
		act = []helpBinding{
			{"Enter", "inspect"},
			{"f", "files"},
			{"d", "remove"},
			{"P", "prune"},
		}
	case TabNetworks:
		act = []helpBinding{
			{"Enter", "inspect"},
			{"d", "remove"},
			{"P", "prune"},
		}
	default:
		act = []helpBinding{
			{"Enter", "inspect"},
			{"d", "remove"},
		}
	}

	return nav + "\n" + renderHelpRow("act", act)
}

func helpFooterPanel(m Model) string {
	switch m.mode {
	case ModeLogs:
		return renderHelpRow("logs", []helpBinding{
			{"/", "find"},
			{"ctrl+g", "regex"},
			{"n/N", "next/prev"},
			{"ctrl+↑↓", "page"},
			{"f", "follow"},
			{"g/G", "top/end"},
			{"esc", "back"},
		})
	case ModeHelp:
		return renderHelpRow("help", []helpBinding{
			{"↑↓", "scroll"},
			{"g/G", "top/end"},
			{"esc", "back"},
			{"q", "quit"},
		})
	default: // ModeDetail
		items := []helpBinding{
			{"l/f", "logs"},
			{"t", "top"},
			{"e", "exec"},
			{"r", "refresh"},
			{"esc", "back"},
			{"q", "quit"},
		}
		if strings.HasPrefix(m.detailTitle, "Volume · ") {
			items = []helpBinding{
				{"f", "files"},
				{"r", "refresh"},
				{"esc", "back"},
				{"q", "quit"},
			}
		}
		return renderHelpRow("detail", items)
	}
}

func helpFooterComposeDetail() string {
	return renderHelpRow("svc", []helpBinding{
		{"↑↓", "select"},
		{"Enter", "inspect"},
		{"l", "logs"},
		{"g", "graphs"},
		{"t", "top"},
		{"e", "exec"},
		{"s", "start"},
		{"x", "stop"},
		{"R", "restart"},
		{"p", "pause"},
		{"k", "kill"},
		{"d", "remove"},
	}) + "\n" + renderHelpRow("prj", []helpBinding{
		{"b", "rebuild"},
		{"D", "remove all"},
		{"c", "containers"},
		{"esc", "back"},
	})
}

func helpFooterWizard(compose bool) string {
	if compose {
		return renderHelpRow("wiz", []helpBinding{
			{"Tab", "fields"},
			{"ctrl+a", "add svc"},
			{"ctrl+w", "remove svc"},
			{"ctrl+y", "YAML"},
			{"ctrl+s", "save"},
			{"ctrl+u", "save+up"},
			{"esc", "back"},
		})
	}
	return renderHelpRow("img", []helpBinding{
		{"Tab", "fields"},
		{"↑↓", "suggest"},
		{"p", "permanent"},
		{"t", "temporary"},
		{"Enter", "run"},
		{"esc", "back"},
	})
}

func helpTextFull() string {
	sec := func(title string) string {
		return helpSectionStyle.Render(title)
	}
	row := func(keys, desc string) string {
		return "  " + helpKeyStyle.Render(fmtPad(keys, 18)) + " " + helpDescStyle.Render(desc)
	}

	var b strings.Builder
	b.WriteString(helpTitleStyle.Render(AppName + " — keyboard reference"))
	b.WriteString("\n\n")

	b.WriteString(sec("Navigation"))
	b.WriteByte('\n')
	b.WriteString(row("1-6  [ ]  Tab", "switch tabs (6 = Settings)"))
	b.WriteByte('\n')
	b.WriteString(row("↑ ↓", "move selection"))
	b.WriteByte('\n')
	b.WriteString(row("Enter / i", "open detail / inspect"))
	b.WriteByte('\n')
	b.WriteString(row("esc", "back / clear filter"))
	b.WriteByte('\n')
	b.WriteString(row("?", "this help"))
	b.WriteByte('\n')
	b.WriteString(row("q", "quit"))
	b.WriteByte('\n')
	b.WriteByte('\n')

	b.WriteString(sec("Settings (tab 6)"))
	b.WriteByte('\n')
	b.WriteString(row("Enter", "open / toggle selected setting"))
	b.WriteByte('\n')
	b.WriteString(row("H", "Docker hosts / favorites"))
	b.WriteByte('\n')
	b.WriteString(row("a / s / d", "add · save current · delete saved"))
	b.WriteByte('\n')
	b.WriteString(row("c", "one-shot custom URL (not saved)"))
	b.WriteByte('\n')
	b.WriteString(row("Remote write", "lock mutations on ssh/tcp hosts (default on)"))
	b.WriteByte('\n')
	b.WriteString(row("Audit log", "view recent mutating actions"))
	b.WriteByte('\n')
	b.WriteString(row("U", "check / install update (SHA256 verified)"))
	b.WriteByte('\n')
	b.WriteByte('\n')

	b.WriteString(sec("Filter & sort"))
	b.WriteByte('\n')
	b.WriteString(row("/", "text filter (space = AND)"))
	b.WriteByte('\n')
	b.WriteString(row("ctrl+u", "clear filter"))
	b.WriteByte('\n')
	b.WriteString(row("o / O", "next sort column / reverse"))
	b.WriteByte('\n')
	b.WriteString(row("F", "running only / volumes: in-use only"))
	b.WriteByte('\n')
	b.WriteString(row("c", "Compose → Containers (project filter)"))
	b.WriteByte('\n')
	b.WriteByte('\n')

	b.WriteString(sec("Lifecycle"))
	b.WriteByte('\n')
	b.WriteString(row("s / x / R", "start / stop / restart"))
	b.WriteByte('\n')
	b.WriteString(row("b", "rebuild (compose up -d --build)"))
	b.WriteByte('\n')
	b.WriteString(row("p / k", "pause·unpause / kill"))
	b.WriteByte('\n')
	b.WriteString(row("d / D", "remove / remove all (compose)"))
	b.WriteByte('\n')
	b.WriteString(row("P", "prune current tab"))
	b.WriteByte('\n')
	b.WriteByte('\n')

	b.WriteString(sec("Inspect & runtime"))
	b.WriteByte('\n')
	b.WriteString(row("l / f", "logs (follow)"))
	b.WriteByte('\n')
	b.WriteString(row("g", "CPU/MEM graphs (history)"))
	b.WriteByte('\n')
	b.WriteString(row("E", "docker events (die/oom/health)"))
	b.WriteByte('\n')
	b.WriteString(row("H", "switch Docker host / favorites"))
	b.WriteByte('\n')
	b.WriteString(row("M", "multi-host side-by-side (Compose/Containers)"))
	b.WriteByte('\n')
	b.WriteString(row("a/s/d (in Hosts)", "add · save current · delete saved"))
	b.WriteByte('\n')
	b.WriteString(row("Tab (in Multi)", "focus left / right pane"))
	b.WriteByte('\n')
	b.WriteString(row("t", "docker top"))
	b.WriteByte('\n')
	b.WriteString(row("e", "docker exec -it"))
	b.WriteByte('\n')
	b.WriteString(row("r", "refresh"))
	b.WriteByte('\n')
	b.WriteString(row("U", "check / install update from GitHub Releases"))
	b.WriteByte('\n')
	b.WriteByte('\n')

	b.WriteString(sec("Create"))
	b.WriteByte('\n')
	b.WriteString(row("n", "new compose (Compose) / new image (Images)"))
	b.WriteByte('\n')
	b.WriteString(row("N", "add image (pull or temporary run --rm)"))
	b.WriteByte('\n')
	b.WriteString(row("ctrl+a/w/y/s/u", "wizard: add / remove / YAML / save / save+up"))
	b.WriteByte('\n')
	b.WriteByte('\n')

	b.WriteString(sec("Volumes"))
	b.WriteByte('\n')
	b.WriteString(row("F", "show in-use volumes only"))
	b.WriteByte('\n')
	b.WriteString(row("f", "open volume file tree (list or detail)"))
	b.WriteByte('\n')
	b.WriteString(row("Enter / Space", "expand dir / preview file"))
	b.WriteByte('\n')
	b.WriteString(row("e", "edit in helix/nvim/$EDITOR (LSP)"))
	b.WriteByte('\n')
	b.WriteString(row("y / Y", "copy / move volume → local"))
	b.WriteByte('\n')
	b.WriteString(row("u / U", "copy / move local → volume"))
	b.WriteByte('\n')
	b.WriteString(row("L", "lint (ruff/go vet/eslint/…)"))
	b.WriteByte('\n')
	b.WriteByte('\n')

	b.WriteString(sec("Logs search"))
	b.WriteByte('\n')
	b.WriteString(row("/", "find (plain text)"))
	b.WriteByte('\n')
	b.WriteString(row("ctrl+g", "find (regex)"))
	b.WriteByte('\n')
	b.WriteString(row("n / N", "next / previous match"))
	b.WriteByte('\n')
	b.WriteString(row("ctrl+↑ / ↓", "page up / down"))
	b.WriteByte('\n')
	b.WriteString(row("g / G", "jump to start / end"))
	b.WriteByte('\n')
	b.WriteByte('\n')

	b.WriteString(helpDescStyle.Render("Auto-refresh ~4s. Ports show published host→container mappings."))
	b.WriteByte('\n')
	return b.String()
}

func fmtPad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
