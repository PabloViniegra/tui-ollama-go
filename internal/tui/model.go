// Package tui implementa la interfaz de terminal con Bubble Tea.
package tui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ollama-fit/internal/eval"
	"ollama-fit/internal/hardware"
)

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

// Update procesa mensajes de Bubble Tea.
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
