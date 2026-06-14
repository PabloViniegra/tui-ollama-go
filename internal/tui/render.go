// Package tui implementa la interfaz de terminal con Bubble Tea.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"ollama-fit/internal/eval"
)

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
	line1 := titleStyle.Render(" Ollama Fit ") + " " +
		hwStyle.Render(fmt.Sprintf("%d modelos", len(m.all))) + " " +
		glyphCounts(g, t, n)
	cpuLine := hwStyle.Render(fmt.Sprintf("CPU  %s · %d núcleos    RAM  %.1f GB",
		m.hw.CPUModel, m.hw.CPUCores, m.hw.RAMGB))
	gpuLine := hwStyle.Render("GPU  " + m.gpuDescr())
	return line1 + "\n" + cpuLine + "\n" + gpuLine + "\n" + m.columnHeader()
}

func cell(s string, w int) string { return lipgloss.NewStyle().Width(w).Render(s) }

func (m Model) columnHeader() string {
	body := gutterGlyph(cDim, false) + " " +
		cell("ESTADO", wStatus) + cell("MODELO", wName) + cell("PARÁM", wParams) +
		cell("CUANT", wQuant) + cell("MEMORIA", wMemory) +
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

func gutterGlyph(c lipgloss.Color, selected bool) string {
	style := lipgloss.NewStyle().Foreground(c)
	if selected {
		style = style.Bold(true)
	}
	return style.Render("▎")
}

func arrowSignature(size, need float64, c lipgloss.Color) string {
	return lipgloss.NewStyle().Foreground(c).Bold(true).Render(fmt.Sprintf("%.1f GB → %.1f GB", size, need))
}

func glyphCounts(g, t, n int) string {
	return lipgloss.NewStyle().Foreground(cGood).Render(fmt.Sprintf("%d ✓", g)) + " " +
		lipgloss.NewStyle().Foreground(cTight).Render(fmt.Sprintf("%d !", t)) + " " +
		lipgloss.NewStyle().Foreground(cNo).Render(fmt.Sprintf("%d ✗", n))
}

func applySelectionStyle(s string, selected bool) string {
	if selected {
		return selRowStyle.Render(s)
	}
	return s
}

func (m Model) renderRow(r eval.Result, selected bool) string {
	nameStyle := lipgloss.NewStyle().Foreground(cName)
	if selected {
		nameStyle = nameStyle.Foreground(cSel).Bold(true)
	}
	var statusCell string
	if selected {
		statusCell = cell(lipgloss.NewStyle().Foreground(statusColor(r.Verdict)).Bold(true).Render(statusText(r.Verdict)), wStatus)
	} else {
		statusCell = cell("", wStatus)
	}
	arrow := arrowSignature(r.Model.SizeGB, r.NeedGB, statusColor(r.Verdict))
	row := gutterGlyph(statusColor(r.Verdict), selected) + " " +
		statusCell +
		cell(nameStyle.Render(r.Model.Name), wName) +
		cell(dimStyle.Render(r.Model.Params), wParams) +
		cell(dimStyle.Render(r.Model.Quant), wQuant) +
		cell(arrow, wMemory) +
		cell(dimStyle.Render(r.Backend), wBackend)
	return applySelectionStyle(row, selected)
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
	return footStyle.Render(fmt.Sprintf("↑/↓ mover · pgup/pgdn · g/G inicio/fin · f filtro [%s] · / buscar · q salir",
		m.filter.label()))
}
