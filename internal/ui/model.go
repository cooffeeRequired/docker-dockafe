package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cooffeeRequired/dockafe/internal/docker"
)

type Tab int

const (
	TabCompose Tab = iota
	TabContainers
	TabImages
	TabVolumes
	TabNetworks
)

const AppName = "Dockafé"

// AppVersion is set at link time via -ldflags "-X …AppVersion=…".
var AppVersion = "1.0.0"

var tabNames = []string{"Compose", "Containers", "Images", "Volumes", "Networks"}

type Mode int

const (
	ModeSplash Mode = iota
	ModeList
	ModeFilter
	ModeDetail
	ModeComposeDetail
	ModeLogs
	ModeHelp
	ModeConfirm
	ModeCreateCompose
	ModePullImage
	ModeVolumeTree
)

type confirmKind int

const (
	confirmNone confirmKind = iota
	confirmRemove
	confirmRemoveAll
	confirmRebuild
	confirmPrune
	confirmKill
	confirmVolWrite
	confirmUpdate
)

type SortKey int

const (
	SortName SortKey = iota
	SortState
	SortCPU
	SortMem
	SortSize
	SortCreated
	SortProject
)

type Model struct {
	client *docker.Client
	width  int
	height int
	ready  bool

	tab    Tab
	mode   Mode
	table  table.Model
	filter textinput.Model
	vp     viewport.Model

	loading     bool
	busy        bool
	errMsg      string
	status      string
	sysInfo     string
	lastSync    time.Time
	runningOnly bool

	splashMinDone   bool
	splashDataReady bool

	sortKey SortKey
	sortAsc bool

	groups     []docker.ComposeGroup
	containers []docker.ContainerInfo
	images     []docker.ImageInfo
	volumes    []docker.VolumeInfo
	networks   []docker.NetworkInfo

	selectedGroup string

	confirm       confirmKind
	confirmTarget string
	confirmLabel  string

	detailTitle string
	detailBody  string
	logsTarget  string
	logsName    string
	logsFollow  bool

	// Log search (/ text, ctrl+g regex — IDE-safe; ctrl+f aliases kept)
	logsSearchOpen    bool
	logsSearchRegex   bool
	logsSearchInput   textinput.Model
	logsSearchQuery   string
	logsSearchMatches []int
	logsSearchIdx     int
	logsSearchErr     string

	// Target for inspect/logs/exec (works across modes)
	targetID   string
	targetName string

	// Interactive compose detail
	composeProject  string
	composeCursor   int
	composeServices []docker.ContainerInfo
	returnToCompose bool

	// Wizards
	cwStep           composeWizardStep
	cwFocus          composeField
	cwInputs         []textinput.Model
	cwYAML           textarea.Model
	cwServices       []docker.ComposeServiceSpec
	iwFocus          imageWizardField
	iwInputs         []textinput.Model
	iwMode           ImageAddMode
	imageSuggestions []string
	acItems          []string
	acIndex          int

	// Volume file tree
	volName        string
	volRoot        *volNode
	volCursor      int
	volOffset      int
	volPreviewPath string
	volPreview     string
	volLint        string
	volErr         string
	volFileFocus   bool
	volPreviewLine int
	volAccessMode  string // "via host" | "via docker"
	volListCache   map[string][]docker.VolumeEntry
	volPendingPath string
	volPendingData []byte
	volReturnMode  Mode // mode to restore after confirm

	updateAvailable bool
	updateLatest    string
	updateURL       string
	updateAssetURL  string
	updateErr       string

	// Async generation tokens (ignore stale responses)
	dataGen uint64
	logsGen uint64
	volGen  uint64
}

type tickMsg time.Time
type logsTickMsg time.Time

type dataMsg struct {
	gen        uint64
	groups     []docker.ComposeGroup
	containers []docker.ContainerInfo
	images     []docker.ImageInfo
	volumes    []docker.VolumeInfo
	networks   []docker.NetworkInfo
	sysInfo    string
	err        error
	at         time.Time
}

type actionDoneMsg struct {
	err error
	msg string
}

type contentMsg struct {
	title      string
	body       string
	mode       Mode
	err        error
	targetID   string
	targetName string
}

type logsMsg struct {
	gen      uint64
	targetID string
	body     string
	err      error
	colored  bool
}

func New(client *docker.Client) Model {
	ti := textinput.New()
	ti.Placeholder = "filter (name, image, port, project…)"
	ti.CharLimit = 120
	ti.Width = 40

	t := table.New(
		table.WithColumns(defaultColumns(TabCompose, 100)),
		table.WithFocused(true),
		table.WithHeight(12),
	)
	t.SetStyles(tableStyles())

	vp := viewport.New(80, 20)
	vp.MouseWheelEnabled = true
	vp.KeyMap = logsKeyMap()

	search := textinput.New()
	search.CharLimit = 200
	search.Width = 40

	return Model{
		client:          client,
		tab:             TabCompose,
		mode:            ModeSplash,
		table:           t,
		filter:          ti,
		vp:              vp,
		logsSearchInput: search,
		loading:         true,
		status:          "loading…",
		sortKey:         SortName,
		sortAsc:         true,
		volListCache:    map[string][]docker.VolumeEntry{},
		dataGen:         1,
	}
}

func logsKeyMap() viewport.KeyMap {
	km := viewport.DefaultKeyMap()
	km.PageUp = key.NewBinding(
		key.WithKeys("pgup", "ctrl+up"),
		key.WithHelp("ctrl+↑/pgup", "page up"),
	)
	km.PageDown = key.NewBinding(
		key.WithKeys("pgdown", "ctrl+down"),
		key.WithHelp("ctrl+↓/pgdn", "page down"),
	)
	km.HalfPageUp = key.NewBinding(
		key.WithKeys("ctrl+u"),
		key.WithHelp("ctrl+u", "½ page up"),
	)
	km.HalfPageDown = key.NewBinding(
		key.WithKeys("ctrl+d"),
		key.WithHelp("ctrl+d", "½ page down"),
	)
	// Don't steal l/f/b from our app bindings when messages fall through.
	km.Up = key.NewBinding(key.WithKeys("up", "k"))
	km.Down = key.NewBinding(key.WithKeys("down", "j"))
	km.Left = key.NewBinding(key.WithKeys("left", "h"))
	km.Right = key.NewBinding(key.WithKeys("right"))
	return km
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.refreshGen(true, m.dataGen), tickCmd(), splashTimerCmd(), splashAnimCmd(), checkUpdateCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(4*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func logsTickCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return logsTickMsg(t)
	})
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("63")).
			Padding(0, 1)
	tabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Padding(0, 1)
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("33")).
			Padding(0, 1)
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243"))
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("110"))
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)
	confirmStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("166")).
			Bold(true).
			Padding(0, 1)
	filterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)
	metaStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))
	updateBadgeStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("232")).
				Background(lipgloss.Color("214")).
				Padding(0, 1)
)

func tableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true).
		Foreground(lipgloss.Color("229"))
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("63")).
		Bold(false)
	return s
}
