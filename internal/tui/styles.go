// Package tui implementa la interfaz de terminal con Bubble Tea.
package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// ---------- palette (dark-terminal-only, truecolor) ----------

var (
	cGood  = lipgloss.Color("#00D787") // green
	cTight = lipgloss.Color("#FFB76B") // amber
	cNo    = lipgloss.Color("#FF5F5F") // red
	cDim   = lipgloss.Color("#6C6C6C") // dim gray
	cName  = lipgloss.Color("#E0E0E0") // model name normal
	cSel   = lipgloss.Color("#FFFFFF") // selected / bright
	cAcc   = lipgloss.Color("#5FD7FF") // accent (title, spinner)
	cSelBg = lipgloss.Color("#1E2A35") // selected row background (teal tint)
	cMuted = lipgloss.Color("#9E9E9E") // column header text
	cFoot  = lipgloss.Color("#585858") // footer / help text
)

// ---------- styles ----------

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(cSel).Background(cAcc).Padding(0, 1)
	selRowStyle  = lipgloss.NewStyle().Background(cSelBg)
	hwStyle      = lipgloss.NewStyle().Foreground(cDim)
	colHeadStyle = lipgloss.NewStyle().Bold(true).Foreground(cMuted)
	footStyle    = lipgloss.NewStyle().Foreground(cFoot).Italic(true)
	dimStyle     = lipgloss.NewStyle().Foreground(cDim)

	nameStyle         = lipgloss.NewStyle().Foreground(cName)
	nameStyleSelected = lipgloss.NewStyle().Foreground(cSel).Bold(true)
	statusStyleBase   = lipgloss.NewStyle().Bold(true)
)

// ---------- column widths ----------

const (
	wStatus  = 9
	wName    = 22
	wParams  = 7
	wQuant   = 10
	wMemory  = 20
	wBackend = 10
)

