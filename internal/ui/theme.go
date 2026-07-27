package ui

import "github.com/charmbracelet/lipgloss"

// Adaptive palette: Light = light terminal background, Dark = dark background.
// Prefer hex so grayscale ANSI (240–252) does not vanish on white terminals.

func adaptive(light, dark string) lipgloss.TerminalColor {
	return lipgloss.AdaptiveColor{Light: light, Dark: dark}
}

var (
	cText      = adaptive("#1a1a1a", "#e8e8e8")
	cMuted     = adaptive("#555555", "#9a9a9a")
	cFaint     = adaptive("#6e6e6e", "#6a6a6a")
	cBorder    = adaptive("#b0b0b0", "#585858")
	cBorderDim = adaptive("#d0d0d0", "#3a3a3a")
	cAccent    = adaptive("#005f87", "#87afd7")
	cWarn      = adaptive("#9a6700", "#ffaf00")
	cError     = adaptive("#c41e3a", "#ff5555")
	cOK        = adaptive("#087f5b", "#87d787")
	cTitleFg   = adaptive("#ffffff", "#ffffd7")
	cTitleBg   = adaptive("#5f5faf", "#5f5faf")
	cTabFg     = adaptive("#3a3a3a", "#8a8a8a")
	cActiveFg  = adaptive("#ffffff", "#ffffd7")
	cActiveBg  = adaptive("#0077c2", "#0087d7")
	cSelFg     = adaptive("#ffffff", "#ffffd7")
	cSelBg     = adaptive("#5f5faf", "#5f5faf")
	cFilterFg  = adaptive("#1a1a1a", "#ffffaf")
	cFilterBg  = adaptive("#e6e6e6", "#303030")
	cHeader    = adaptive("#4b0082", "#ffffaf")
	cHelpKey   = adaptive("#875f00", "#ffffaf")
	cHelpSec   = adaptive("#005f87", "#87d7ff")
	cConfirmFg = adaptive("#ffffff", "#ffffd7")
	cConfirmBg = adaptive("#d9480f", "#d75f00")
	cBadgeFg   = adaptive("#1a1a1a", "#1a1a1a")
	cBadgeBg   = adaptive("#e67700", "#ffaf00")
	cCPU       = adaptive("#0b7285", "#00d7ff")
	cMem       = adaptive("#e67700", "#ffd75f")
	cHostCPU   = adaptive("#0b7285", "#2dd4bf")
	cHostMem   = adaptive("#1c7ed6", "#4dabf7")
	cDockCPU   = adaptive("#c92a2a", "#ff6b6b")
	cDockMem   = adaptive("#7048e8", "#b197fc")
	cDisk      = adaptive("#e67700", "#fcc419")
	cBright    = adaptive("#111111", "#ffffff")
	cLogMsg    = adaptive("#1a1a1a", "#e4e4e4")
	cLogTS     = adaptive("#495057", "#5f8787")
	cLogDebug  = adaptive("#868e96", "#6a6a6a")
	cWhale     = adaptive("#0077c2", "#00afd7")
	cCoffee    = adaptive("#a15c38", "#d7af87")
	cSearchFg  = adaptive("#ffffff", "#ffffd7")
	cSearchBg  = adaptive("#5f5faf", "#5f5faf")
	cSearchRow = adaptive("#dee2e6", "#444444")
)

// Container name colors that stay readable on light and dark backgrounds.
var containerColors = []lipgloss.TerminalColor{
	adaptive("#0077c2", "#00afd7"),
	adaptive("#d9480f", "#ff8700"),
	adaptive("#5f3dc4", "#af87d7"),
	adaptive("#087f5b", "#5fd75f"),
	adaptive("#c2255c", "#ff5f87"),
	adaptive("#0b7285", "#00d7d7"),
	adaptive("#e67700", "#ffd75f"),
	adaptive("#9c36b5", "#d787d7"),
	adaptive("#0c8599", "#5fafd7"),
	adaptive("#e03131", "#ff8787"),
}
