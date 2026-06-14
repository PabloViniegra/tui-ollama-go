// Package tui implementa la interfaz de terminal con Bubble Tea.
package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// ---------- estilos ----------

var (
	cGood  = lipgloss.Color("42")  // verde
	cTight = lipgloss.Color("214") // ámbar
	cNo    = lipgloss.Color("203") // rojo
	cDim   = lipgloss.Color("244")
	cName  = lipgloss.Color("252")
	cSel   = lipgloss.Color("231")
	cAcc   = lipgloss.Color("39")
	cHdrBg = lipgloss.Color("236")
	cSelBg = lipgloss.Color("237")

	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(cSel).Background(cAcc)
	selRowStyle  = lipgloss.NewStyle().Background(cSelBg)
	hwStyle      = lipgloss.NewStyle().Foreground(cDim)
	colHeadStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("250")).Background(cHdrBg)
	footStyle    = lipgloss.NewStyle().Foreground(cDim)
	dimStyle     = lipgloss.NewStyle().Foreground(cDim)

	nameStyle         = lipgloss.NewStyle().Foreground(cName)
	nameStyleSelected = lipgloss.NewStyle().Foreground(cSel).Bold(true)
	statusStyleBase   = lipgloss.NewStyle().Bold(true)
)

// anchos de columna
const (
	wStatus  = 9
	wName    = 24
	wParams  = 7
	wQuant   = 10
	wMemory  = 17
	wBackend = 12
)

// ---------- filtro ----------

type filter int

const (
	fAll filter = iota
	fGood
	fTight
	fNo
)

func (f filter) label() string {
	switch f {
	case fGood:
		return msgFilterGood
	case fTight:
		return msgFilterTight
	case fNo:
		return msgFilterNo
	default:
		return msgFilterAll
	}
}
