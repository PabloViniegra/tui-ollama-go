// Package tui implementa la interfaz de terminal con Bubble Tea.
package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ollama-fit/internal/eval"
	"ollama-fit/internal/hardware"
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

	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(cSel).Background(cAcc)
	hwStyle      = lipgloss.NewStyle().Foreground(cDim)
	colHeadStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("250")).Background(cHdrBg)
	footStyle    = lipgloss.NewStyle().Foreground(cDim)
	dimStyle     = lipgloss.NewStyle().Foreground(cDim)

	normGutter = "  "
	selGutter  = lipgloss.NewStyle().Foreground(cAcc).Bold(true).Render("▌ ")
)

// anchos de columna
const (
	wStatus  = 9
	wName    = 24
	wParams  = 7
	wQuant   = 10
	wSize    = 9
	wNeed    = 10
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
		return "va bien"
	case fTight:
		return "justo"
	case fNo:
		return "no cabe"
	default:
		return "todos"
	}
}

// ---------- modelo ----------

// Model es el estado de la TUI.
type Model struct {
	hw        hardware.Info
	all       []eval.Result
	view      []eval.Result
	cursor    int
	offset    int
	width     int
	height    int
	filter    filter
	search    string
	searching bool
}

// New construye el modelo ordenando por tamaño ascendente.
func New(hw hardware.Info, results []eval.Result) Model {
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Model.SizeGB < results[j].Model.SizeGB
	})
	m := Model{hw: hw, all: results, filter: fAll}
	m.applyFilter()
	return m
}

func (m Model) Init() tea.Cmd { return nil }

func (m *Model) applyFilter() {
	q := strings.ToLower(strings.TrimSpace(m.search))
	var out []eval.Result
	for _, r := range m.all {
		switch m.filter {
		case fGood:
			if r.Verdict != eval.Good {
				continue
			}
		case fTight:
			if r.Verdict != eval.Tight {
				continue
			}
		case fNo:
			if r.Verdict != eval.No {
				continue
			}
		}
		if q != "" && !strings.Contains(strings.ToLower(r.Model.Name), q) {
			continue
		}
		out = append(out, r)
	}
	m.view = out
	if m.cursor >= len(m.view) {
		m.cursor = max(0, len(m.view)-1)
	}
	m.clampOffset()
}

func (m Model) listHeight() int {
	h := m.height - lipgloss.Height(m.header()) - lipgloss.Height(m.footer())
	if h < 1 {
		h = 1
	}
	return h
}

func (m *Model) clampOffset() {
	lh := m.listHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+lh {
		m.offset = m.cursor - lh + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
	if maxOff := len(m.view) - lh; m.offset > maxOff {
		if maxOff < 0 {
			maxOff = 0
		}
		m.offset = maxOff
	}
}

// ---------- update ----------

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampOffset()
		return m, nil
	case tea.KeyMsg:
		if m.searching {
			return m.updateSearch(msg)
		}
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.view)-1 {
				m.cursor++
			}
		case "pgup":
			m.cursor -= m.listHeight()
			if m.cursor < 0 {
				m.cursor = 0
			}
		case "pgdown":
			m.cursor += m.listHeight()
			if m.cursor > len(m.view)-1 {
				m.cursor = len(m.view) - 1
			}
		case "home", "g":
			m.cursor = 0
		case "end", "G":
			m.cursor = len(m.view) - 1
		case "f":
			m.filter = (m.filter + 1) % 4
			m.cursor = 0
			m.applyFilter()
		case "/":
			m.searching = true
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.clampOffset()
	}
	return m, nil
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.searching = false
	case "esc":
		m.searching = false
		m.search = ""
		m.applyFilter()
	case "backspace":
		if len(m.search) > 0 {
			m.search = m.search[:len(m.search)-1]
			m.applyFilter()
		}
	default:
		if len(msg.String()) == 1 {
			m.search += msg.String()
			m.applyFilter()
		}
	}
	return m, nil
}

// ---------- view ----------

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Detectando hardware…"
	}
	return m.header() + "\n" + m.listView(m.listHeight()) + "\n" + m.footer()
}

func (m Model) counts() (g, t, n int) {
	for _, r := range m.all {
		switch r.Verdict {
		case eval.Good:
			g++
		case eval.Tight:
			t++
		default:
			n++
		}
	}
	return
}

func (m Model) gpuDescr() string {
	g := m.hw.GPU
	switch {
	case g.Kind == "apple":
		return fmt.Sprintf("%s · memoria unificada (~%.0f GB usables por GPU)", g.Name, 0.70*m.hw.RAMGB)
	case g.VRAMGB > 0:
		return fmt.Sprintf("%s · %.1f GB VRAM (%s)", g.Name, g.VRAMGB, strings.ToUpper(g.Kind))
	case g.Kind == "" || g.Kind == "none" || g.Name == "":
		return "sin GPU dedicada · inferencia en CPU"
	default:
		return fmt.Sprintf("%s · VRAM desconocida", g.Name)
	}
}

func (m Model) header() string {
	g, t, n := m.counts()
	chip := func(c lipgloss.Color, k int) string {
		return lipgloss.NewStyle().Foreground(c).Render(fmt.Sprintf("●%d", k))
	}
	line1 := titleStyle.Render(" Ollama Fit ") + "  " +
		hwStyle.Render(fmt.Sprintf("%s/%s · %d modelos", m.hw.OS, m.hw.Arch, len(m.all))) + "   " +
		chip(cGood, g) + " " + chip(cTight, t) + " " + chip(cNo, n)
	cpuLine := hwStyle.Render(fmt.Sprintf("CPU  %s · %d núcleos    RAM  %.1f GB",
		m.hw.CPUModel, m.hw.CPUCores, m.hw.RAMGB))
	gpuLine := hwStyle.Render("GPU  " + m.gpuDescr())
	return line1 + "\n" + cpuLine + "\n" + gpuLine + "\n\n" + m.columnHeader()
}

func cell(s string, w int) string { return lipgloss.NewStyle().Width(w).Render(s) }

func (m Model) columnHeader() string {
	body := normGutter +
		cell("ESTADO", wStatus) + cell("MODELO", wName) + cell("PARÁM", wParams) +
		cell("CUANT", wQuant) + cell("TAMAÑO", wSize) + cell("NECESITA", wNeed) +
		cell("BACKEND", wBackend)
	return colHeadStyle.Render(body)
}

func statusText(v eval.Verdict) string {
	switch v {
	case eval.Good:
		return "Va bien"
	case eval.Tight:
		return "Justo"
	default:
		return "No cabe"
	}
}

func statusColor(v eval.Verdict) lipgloss.Color {
	switch v {
	case eval.Good:
		return cGood
	case eval.Tight:
		return cTight
	default:
		return cNo
	}
}

func (m Model) renderRow(r eval.Result, selected bool) string {
	nameStyle := lipgloss.NewStyle().Foreground(cName)
	if selected {
		nameStyle = nameStyle.Foreground(cSel).Bold(true)
	}
	statusCell := cell(lipgloss.NewStyle().Foreground(statusColor(r.Verdict)).Bold(true).Render(statusText(r.Verdict)), wStatus)
	row := statusCell +
		cell(nameStyle.Render(r.Model.Name), wName) +
		cell(dimStyle.Render(r.Model.Params), wParams) +
		cell(dimStyle.Render(r.Model.Quant), wQuant) +
		cell(dimStyle.Render(fmt.Sprintf("%.1f GB", r.Model.SizeGB)), wSize) +
		cell(dimStyle.Render(fmt.Sprintf("%.1f GB", r.NeedGB)), wNeed) +
		cell(dimStyle.Render(r.Backend), wBackend)
	gutter := normGutter
	if selected {
		gutter = selGutter
	}
	return gutter + row
}

func (m Model) listView(height int) string {
	if len(m.view) == 0 {
		return dimStyle.Render("  (sin resultados para el filtro/búsqueda actual)")
	}
	start := m.offset
	end := start + height
	if end > len(m.view) {
		end = len(m.view)
	}
	lines := make([]string, 0, height)
	for i := start; i < end; i++ {
		lines = append(lines, m.renderRow(m.view[i], i == m.cursor))
	}
	for len(lines) < height { // relleno para layout estable
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m Model) footer() string {
	if m.searching {
		return footStyle.Render(fmt.Sprintf(" buscar: %s_   (enter aplica · esc limpia)", m.search))
	}
	legend := lipgloss.NewStyle().Foreground(cGood).Render("● va bien") + "  " +
		lipgloss.NewStyle().Foreground(cTight).Render("● justo") + "  " +
		lipgloss.NewStyle().Foreground(cNo).Render("● no cabe")
	keys := footStyle.Render(fmt.Sprintf("↑/↓ mover · pgup/pgdn · g/G inicio/fin · f filtro [%s] · / buscar · q salir",
		m.filter.label()))
	return legend + "\n" + keys
}
