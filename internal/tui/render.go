// Package tui implementa la interfaz de terminal con Bubble Tea.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/PabloViniegra/tui-ollama-go/internal/eval"
	"github.com/PabloViniegra/tui-ollama-go/internal/hardware"
)

// ponytail: listHeight() subtracts this — View() owns the count, model.go references it.
const separatorLines = 2

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func spinnerFrame(tick int) string {
	return spinnerFrames[tick%len(spinnerFrames)]
}

func (m Model) separator() string {
	return dimStyle.Render(strings.Repeat("─", m.width))
}

func (m Model) View() string {
	if m.loading || m.width == 0 || m.height == 0 {
		frame := lipgloss.NewStyle().Foreground(cAcc).Render(spinnerFrame(m.spinnerTick))
		return frame + " " + MsgDetectingHardware
	}
	sep := m.separator()
	return m.header() + "\n" + sep + "\n" + m.listView(m.listHeight()) + "\n" + sep + "\n" + m.footer()
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
	case g.Kind == hardware.GPUKindApple:
		return fmt.Sprintf("%s · memoria unificada (~%.0f GB usables por GPU)", g.Name, eval.AppleGPUFraction*m.hw.RAMGB)
	case g.VRAMGB > 0:
		return fmt.Sprintf("%s · %.1f GB VRAM (%s)", g.Name, g.VRAMGB, strings.ToUpper(g.Kind.String()))
	case g.Kind == "" || g.Kind == hardware.GPUKindNone || g.Name == "":
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
	cpuLine := hwStyle.MaxWidth(m.width).Render(fmt.Sprintf(msgCPUHeader,
		m.hw.CPUModel, m.hw.CPUCores, m.hw.RAMGB))
	gpuLine := hwStyle.MaxWidth(m.width).Render(fmt.Sprintf(msgGPUHeader, m.gpuDescr()))
	return line1 + "\n" + cpuLine + "\n" + gpuLine + "\n" + m.columnHeader()
}

// cell trunca visualmente s a w columnas (con elipsis) y luego paddea hasta w.
// Garantiza que la celda tenga exactamente w columnas visibles — sin esto, un
// nombre de modelo largo o un arrow "X.X GB → Y.Y GB" ancho hace crecer la
// celda, la fila se pasa de m.width y MaxWidth la envuelve a 2+ líneas,
// rompiendo el alineamiento de columnas y desbordando la View.
func cell(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if ansi.StringWidth(s) > w {
		s = ansi.Truncate(s, w, "…")
	}
	return lipgloss.NewStyle().Width(w).Render(s)
}

func (m Model) columnHeader() string {
	body := gutterGlyph(cDim, false) + " " +
		cell(msgStatus, wStatus) + cell(msgModel, wName) + cell(msgParams, wParams) +
		cell(msgQuant, wQuant) + cell(msgMemory, wMemory) +
		cell(msgBackend, wBackend)
	return colHeadStyle.MaxWidth(m.width).Render(body)
}

func statusText(v eval.Verdict) string {
	switch v {
	case eval.Good:
		return msgGood
	case eval.Tight:
		return msgTight
	default:
		return msgNoFit
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
	ns := nameStyle
	if selected {
		ns = nameStyleSelected
	}
	var statusCell string
	if selected {
		statusCell = cell(statusStyleBase.Foreground(statusColor(r.Verdict)).Render(statusText(r.Verdict)), wStatus)
	} else {
		statusCell = cell("", wStatus)
	}
	arrow := arrowSignature(r.Model.SizeGB, r.NeedGB, statusColor(r.Verdict))
	row := gutterGlyph(statusColor(r.Verdict), selected) + " " +
		statusCell +
		cell(ns.Render(r.Model.Name), wName) +
		cell(dimStyle.Render(r.Model.Params), wParams) +
		cell(dimStyle.Render(r.Model.Quant), wQuant) +
		cell(arrow, wMemory) +
		cell(dimStyle.Render(r.Backend), wBackend)
	return applySelectionStyle(row, selected)
}

func (m Model) listView(height int) string {
	if len(m.view) == 0 {
		return dimStyle.Render(msgNoResults)
	}
	start := m.offset
	end := min(start+height, len(m.view))
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
		return footStyle.Render(fmt.Sprintf(msgSearchPrompt, m.search))
	}
	if m.message != "" {
		return footStyle.Render(m.message)
	}
	return footStyle.Render(fmt.Sprintf(msgFooterHelp, m.filter.label()))
}
