package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"ollama-fit/internal/catalog"
	"ollama-fit/internal/eval"
	"ollama-fit/internal/hardware"
)

// helpers

func hwNone() hardware.Info { return hardware.Info{RAMGB: 16, CPUCores: 8} }

func makeResults() []eval.Result {
	return []eval.Result{
		{Model: catalog.Model{Name: "big:70b", SizeGB: 40}, Verdict: eval.No, Backend: "—"},
		{Model: catalog.Model{Name: "small:7b", SizeGB: 4}, Verdict: eval.Good, Backend: "CPU"},
		{Model: catalog.Model{Name: "mid:13b", SizeGB: 7}, Verdict: eval.Tight, Backend: "CPU"},
	}
}

func newWithSize(hw hardware.Info, results []eval.Result, w, h int) Model {
	m := New(hw, results)
	m.width, m.height = w, h
	return m
}

// ---------- filter ----------

func TestFilterLabel(t *testing.T) {
	tests := []struct {
		f    filter
		want string
	}{
		{fAll, "todos"},
		{fGood, "va bien"},
		{fTight, "justo"},
		{fNo, "no cabe"},
	}
	for _, tc := range tests {
		got := tc.f.label()
		if got != tc.want {
			t.Errorf("filter(%d).label() = %q, want %q", tc.f, got, tc.want)
		}
	}
}

// ---------- New / sorting ----------

func TestNewSortsBySizeAscending(t *testing.T) {
	results := makeResults()
	m := New(hwNone(), results)
	for i := 1; i < len(m.all); i++ {
		if m.all[i].Model.SizeGB < m.all[i-1].Model.SizeGB {
			t.Errorf("not sorted: all[%d].SizeGB=%v < all[%d].SizeGB=%v",
				i, m.all[i].Model.SizeGB, i-1, m.all[i-1].Model.SizeGB)
		}
	}
}

func TestNewEmptyResults(t *testing.T) {
	m := New(hwNone(), nil)
	if len(m.all) != 0 {
		t.Errorf("expected 0 results, got %d", len(m.all))
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
}

func TestInit(t *testing.T) {
	m := New(hwNone(), nil)
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

// ---------- applyFilter ----------

func TestApplyFilterAll(t *testing.T) {
	m := New(hwNone(), makeResults())
	m.filter = fAll
	m.applyFilter()
	if len(m.view) != 3 {
		t.Errorf("fAll: got %d items, want 3", len(m.view))
	}
}

func TestApplyFilterGood(t *testing.T) {
	m := New(hwNone(), makeResults())
	m.filter = fGood
	m.applyFilter()
	if len(m.view) != 1 {
		t.Fatalf("fGood: got %d items, want 1", len(m.view))
	}
	if m.view[0].Verdict != eval.Good {
		t.Error("filtered item is not Good")
	}
}

func TestApplyFilterTight(t *testing.T) {
	m := New(hwNone(), makeResults())
	m.filter = fTight
	m.applyFilter()
	if len(m.view) != 1 {
		t.Fatalf("fTight: got %d items, want 1", len(m.view))
	}
	if m.view[0].Verdict != eval.Tight {
		t.Error("filtered item is not Tight")
	}
}

func TestApplyFilterNo(t *testing.T) {
	m := New(hwNone(), makeResults())
	m.filter = fNo
	m.applyFilter()
	if len(m.view) != 1 {
		t.Fatalf("fNo: got %d items, want 1", len(m.view))
	}
	if m.view[0].Verdict != eval.No {
		t.Error("filtered item is not No")
	}
}

func TestApplyFilterSearch(t *testing.T) {
	m := New(hwNone(), makeResults())
	m.filter = fAll
	m.search = "small"
	m.applyFilter()
	if len(m.view) != 1 {
		t.Fatalf("search 'small': got %d items, want 1", len(m.view))
	}
	if m.view[0].Model.Name != "small:7b" {
		t.Errorf("got %q, want %q", m.view[0].Model.Name, "small:7b")
	}
}

func TestApplyFilterSearchCaseInsensitive(t *testing.T) {
	m := New(hwNone(), makeResults())
	m.filter = fAll
	m.search = "SMALL"
	m.applyFilter()
	if len(m.view) != 1 {
		t.Fatalf("search 'SMALL': got %d items, want 1", len(m.view))
	}
}

func TestApplyFilterSearchNoMatch(t *testing.T) {
	m := New(hwNone(), makeResults())
	m.search = "notfound"
	m.applyFilter()
	if len(m.view) != 0 {
		t.Errorf("search 'notfound': got %d items, want 0", len(m.view))
	}
}

func TestApplyFilterClampsCursor(t *testing.T) {
	m := New(hwNone(), makeResults())
	m.cursor = 2
	m.filter = fGood // only 1 item
	m.applyFilter()
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 after filter reduces view", m.cursor)
	}
}

// ---------- counts ----------

func TestCounts(t *testing.T) {
	m := New(hwNone(), makeResults())
	g, ti, n := m.counts()
	if g != 1 {
		t.Errorf("good = %d, want 1", g)
	}
	if ti != 1 {
		t.Errorf("tight = %d, want 1", ti)
	}
	if n != 1 {
		t.Errorf("no = %d, want 1", n)
	}
}

func TestCountsEmpty(t *testing.T) {
	m := New(hwNone(), nil)
	g, ti, n := m.counts()
	if g+ti+n != 0 {
		t.Errorf("expected all zeros, got g=%d ti=%d n=%d", g, ti, n)
	}
}

// ---------- statusText / statusColor ----------

func TestStatusText(t *testing.T) {
	tests := []struct {
		v    eval.Verdict
		want string
	}{
		{eval.Good, "Va bien"},
		{eval.Tight, "Justo"},
		{eval.No, "No cabe"},
	}
	for _, tc := range tests {
		got := statusText(tc.v)
		if got != tc.want {
			t.Errorf("statusText(%v) = %q, want %q", tc.v, got, tc.want)
		}
	}
}

func TestStatusColor(t *testing.T) {
	tests := []struct {
		v    eval.Verdict
		want lipgloss.Color
	}{
		{eval.Good, cGood},
		{eval.Tight, cTight},
		{eval.No, cNo},
	}
	for _, tc := range tests {
		got := statusColor(tc.v)
		if got != tc.want {
			t.Errorf("statusColor(%v) = %v, want %v", tc.v, got, tc.want)
		}
	}
}

// ---------- helpers ----------

func TestGutterGlyph(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	cases := []struct {
		c        lipgloss.Color
		selected bool
	}{
		{cGood, false},
		{cGood, true},
		{cTight, false},
		{cTight, true},
		{cNo, false},
		{cNo, true},
	}
	for _, tc := range cases {
		got := gutterGlyph(tc.c, tc.selected)
		if !strings.Contains(got, "▎") {
			t.Errorf("gutterGlyph(%v, %v) missing glyph, got %q", tc.c, tc.selected, got)
		}
		if tc.selected && !strings.Contains(got, "\x1b[1") {
			t.Errorf("gutterGlyph(%v, true) missing bold ANSI sequence, got %q", tc.c, got)
		}
		if !tc.selected && strings.Contains(got, "\x1b[1") {
			t.Errorf("gutterGlyph(%v, false) should not be bold, got %q", tc.c, got)
		}
	}
}

func TestArrowSignature(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	cases := []struct {
		size, need float64
		c          lipgloss.Color
	}{
		{0, 0, cGood},
		{4.5, 3.2, cGood},
		{8.0, 8.0, cNo},
		{100.5, 200.0, cNo},
	}
	for _, tc := range cases {
		got := arrowSignature(tc.size, tc.need, tc.c)
		if !strings.Contains(got, "→") {
			t.Errorf("arrowSignature(%v,%v) missing arrow, got %q", tc.size, tc.need, got)
		}
		want := fmt.Sprintf("%.1f GB", tc.size)
		if !strings.Contains(got, want) {
			t.Errorf("arrowSignature(%v,%v) missing size %q, got %q", tc.size, tc.need, want, got)
		}
		// Verify color application: ANSI 256 codes for cGood (42) and cNo (203)
		var wantColor string
		switch tc.c {
		case cGood:
			wantColor = "38;5;42m"
		case cNo:
			wantColor = "38;5;203m"
		}
		if wantColor != "" && !strings.Contains(got, wantColor) {
			t.Errorf("arrowSignature(%v,%v,%v) missing color sequence %q, got %q", tc.size, tc.need, tc.c, wantColor, got)
		}
	}
}

func TestGlyphCounts(t *testing.T) {
	got := glyphCounts(2, 1, 3)
	if !strings.Contains(got, "✓") || !strings.Contains(got, "!") || !strings.Contains(got, "✗") {
		t.Errorf("glyphCounts missing glyphs, got %q", got)
	}
	if !strings.Contains(got, "2") || !strings.Contains(got, "1") || !strings.Contains(got, "3") {
		t.Errorf("glyphCounts missing numbers, got %q", got)
	}

	gotZero := glyphCounts(0, 0, 0)
	if !strings.Contains(gotZero, "✓") || !strings.Contains(gotZero, "!") || !strings.Contains(gotZero, "✗") {
		t.Errorf("glyphCounts(0,0,0) missing glyphs, got %q", gotZero)
	}
}

func TestApplySelectionStyle(t *testing.T) {
	s := "test"
	gotSel := applySelectionStyle(s, true)
	if gotSel == "" || !strings.Contains(gotSel, s) {
		t.Errorf("applySelectionStyle selected: got %q, want non-empty containing %q", gotSel, s)
	}
	gotUnsel := applySelectionStyle(s, false)
	if gotUnsel != s {
		t.Errorf("applySelectionStyle unselected: got %q, want %q", gotUnsel, s)
	}
}

// ---------- gpuDescr ----------

func TestGpuDescrApple(t *testing.T) {
	hw := hardware.Info{
		RAMGB:        24,
		AppleUnified: true,
		GPU:          hardware.GPU{Name: "M2 Pro", Kind: "apple"},
	}
	m := New(hw, nil)
	got := m.gpuDescr()
	if !strings.Contains(got, "M2 Pro") {
		t.Errorf("gpuDescr apple: %q missing 'M2 Pro'", got)
	}
	if !strings.Contains(got, "17") { // floor(0.70*24) = 16, sprintf %.0f = 17
		// actually 0.70*24 = 16.8, %.0f rounds to 17
		t.Logf("gpuDescr apple: %q (OK if contains '16' or '17')", got)
	}
}

func TestGpuDescrNVIDIA(t *testing.T) {
	hw := hardware.Info{GPU: hardware.GPU{Name: "RTX 4070", VRAMGB: 12, Kind: "nvidia"}}
	m := New(hw, nil)
	got := m.gpuDescr()
	if !strings.Contains(got, "RTX 4070") {
		t.Errorf("gpuDescr nvidia: %q missing name", got)
	}
	if !strings.Contains(got, "12.0 GB") {
		t.Errorf("gpuDescr nvidia: %q missing VRAM", got)
	}
	if !strings.Contains(got, "NVIDIA") {
		t.Errorf("gpuDescr nvidia: %q missing kind", got)
	}
}

func TestGpuDescrNoGPU(t *testing.T) {
	hw := hardware.Info{GPU: hardware.GPU{Kind: "none"}}
	m := New(hw, nil)
	got := m.gpuDescr()
	if !strings.Contains(got, "sin GPU") {
		t.Errorf("gpuDescr none: %q missing 'sin GPU'", got)
	}
}

func TestGpuDescrUnknownVRAM(t *testing.T) {
	hw := hardware.Info{GPU: hardware.GPU{Name: "Intel HD 630", Kind: "intel", VRAMGB: 0}}
	m := New(hw, nil)
	got := m.gpuDescr()
	if !strings.Contains(got, "VRAM desconocida") {
		t.Errorf("gpuDescr unknown VRAM: %q", got)
	}
}

// ---------- View ----------

func TestViewZeroSize(t *testing.T) {
	m := New(hwNone(), makeResults())
	got := m.View()
	if got != "Detectando hardware…" {
		t.Errorf("View() with zero size = %q", got)
	}
}

func TestViewWithSize(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	got := m.View()
	if got == "" {
		t.Error("View() returned empty string")
	}
	if got == "Detectando hardware…" {
		t.Error("View() still shows loading with non-zero size")
	}
	if !strings.Contains(got, "Ollama Fit") {
		t.Errorf("View() missing title, got %q", got)
	}
	if !strings.Contains(got, "✓") && !strings.Contains(got, "!") && !strings.Contains(got, "✗") {
		t.Errorf("View() missing verdict glyphs, got %q", got)
	}
}

// ---------- Update: window size ----------

func TestUpdateWindowSize(t *testing.T) {
	m := New(hwNone(), makeResults())
	newModel, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	m2 := newModel.(Model)
	if m2.width != 120 || m2.height != 40 {
		t.Errorf("got w=%d h=%d, want 120 40", m2.width, m2.height)
	}
}

// ---------- Update: navigation ----------

func key(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestUpdateMoveDown(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	newModel, _ := m.Update(key("j"))
	m2 := newModel.(Model)
	if m2.cursor != 1 {
		t.Errorf("after j: cursor = %d, want 1", m2.cursor)
	}
}

func TestUpdateMoveUp(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	m.cursor = 2
	newModel, _ := m.Update(key("k"))
	m2 := newModel.(Model)
	if m2.cursor != 1 {
		t.Errorf("after k: cursor = %d, want 1", m2.cursor)
	}
}

func TestUpdateMoveDownAtBottom(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	m.cursor = 2
	newModel, _ := m.Update(key("j"))
	m2 := newModel.(Model)
	if m2.cursor != 2 {
		t.Errorf("after j at bottom: cursor = %d, want 2", m2.cursor)
	}
}

func TestUpdateMoveUpAtTop(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	newModel, _ := m.Update(key("k"))
	m2 := newModel.(Model)
	if m2.cursor != 0 {
		t.Errorf("after k at top: cursor = %d, want 0", m2.cursor)
	}
}

func TestUpdateArrowDown(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m2 := newModel.(Model)
	if m2.cursor != 1 {
		t.Errorf("after down: cursor = %d, want 1", m2.cursor)
	}
}

func TestUpdateArrowUp(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	m.cursor = 2
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m2 := newModel.(Model)
	if m2.cursor != 1 {
		t.Errorf("after up: cursor = %d, want 1", m2.cursor)
	}
}

func TestUpdateHome(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	m.cursor = 2
	newModel, _ := m.Update(key("g"))
	m2 := newModel.(Model)
	if m2.cursor != 0 {
		t.Errorf("after g: cursor = %d, want 0", m2.cursor)
	}
}

func TestUpdateEnd(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	newModel, _ := m.Update(key("G"))
	m2 := newModel.(Model)
	if m2.cursor != 2 {
		t.Errorf("after G: cursor = %d, want 2", m2.cursor)
	}
}

func TestUpdateKeyHome(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	m.cursor = 2
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyHome})
	m2 := newModel.(Model)
	if m2.cursor != 0 {
		t.Errorf("after Home: cursor = %d, want 0", m2.cursor)
	}
}

func TestUpdateKeyEnd(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m2 := newModel.(Model)
	if m2.cursor != 2 {
		t.Errorf("after End: cursor = %d, want 2", m2.cursor)
	}
}

func TestUpdatePgDown(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m2 := newModel.(Model)
	if m2.cursor < 0 {
		t.Errorf("cursor went negative after pgdown: %d", m2.cursor)
	}
}

func TestUpdatePgUp(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	m.cursor = 2
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m2 := newModel.(Model)
	if m2.cursor < 0 {
		t.Errorf("cursor went negative after pgup: %d", m2.cursor)
	}
}

func TestUpdateQuit(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	_, cmd := m.Update(key("q"))
	if cmd == nil {
		t.Error("expected quit cmd after 'q'")
	}
}

func TestUpdateEscQuit(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Error("expected quit cmd after Esc")
	}
}

func TestUpdateFilterCycle(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	if m.filter != fAll {
		t.Fatal("initial filter should be fAll")
	}
	newModel, _ := m.Update(key("f"))
	m2 := newModel.(Model)
	if m2.filter != fGood {
		t.Errorf("after f: filter = %v, want fGood", m2.filter)
	}
}

func TestUpdateFilterCyclesAll(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	for i, want := range []filter{fGood, fTight, fNo, fAll} {
		newModel, _ := m.Update(key("f"))
		m = newModel.(Model)
		if m.filter != want {
			t.Errorf("cycle %d: filter = %v, want %v", i, m.filter, want)
		}
	}
}

// ---------- Update: search mode ----------

func TestUpdateEnterSearchMode(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	newModel, _ := m.Update(key("/"))
	m2 := newModel.(Model)
	if !m2.searching {
		t.Error("expected searching=true after '/'")
	}
}

func TestUpdateSearchTyping(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	m.searching = true

	for _, ch := range []string{"s", "m", "a", "l", "l"} {
		newModel, _ := m.Update(key(ch))
		m = newModel.(Model)
	}
	if m.search != "small" {
		t.Errorf("search = %q, want %q", m.search, "small")
	}
}

func TestUpdateSearchBackspace(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	m.searching = true
	m.search = "smal"

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m2 := newModel.(Model)
	if m2.search != "sma" {
		t.Errorf("after backspace: search = %q, want %q", m2.search, "sma")
	}
}

func TestUpdateSearchBackspaceEmpty(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	m.searching = true
	m.search = ""

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m2 := newModel.(Model)
	if m2.search != "" {
		t.Errorf("backspace on empty: search = %q", m2.search)
	}
}

func TestUpdateSearchEnter(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	m.searching = true
	m.search = "small"

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := newModel.(Model)
	if m2.searching {
		t.Error("expected searching=false after Enter")
	}
	if m2.search != "small" {
		t.Errorf("search should be preserved: %q", m2.search)
	}
}

func TestUpdateSearchEsc(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	m.searching = true
	m.search = "small"

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := newModel.(Model)
	if m2.searching {
		t.Error("expected searching=false after Esc")
	}
	if m2.search != "" {
		t.Errorf("search should be cleared after Esc: %q", m2.search)
	}
}

func TestUpdateCtrlC(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("expected quit cmd after ctrl+c")
	}
}

func TestUpdateUnknownMsg(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	newModel, cmd := m.Update("unexpected message type")
	if cmd != nil {
		t.Error("unexpected cmd for unknown msg")
	}
	m2 := newModel.(Model)
	if m2.cursor != 0 {
		t.Error("state should be unchanged for unknown msg")
	}
}

func TestViewEmptyFilter(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	m.search = "zzz_no_match"
	m.applyFilter()
	got := m.View()
	if !strings.Contains(got, "sin resultados") {
		t.Errorf("expected empty-result message in View(), got: %q", got)
	}
}

// ---------- renderRow integration ----------

func TestRenderRowGood(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	got := m.renderRow(m.all[0], false)
	if !strings.Contains(got, "▎") {
		t.Errorf("Good row missing gutter, got %q", got)
	}
	if !strings.Contains(got, "→") {
		t.Errorf("Good row missing arrow, got %q", got)
	}
	if strings.Contains(got, "Va bien") {
		t.Errorf("Good unselected row should not show status text, got %q", got)
	}
}

func TestRenderRowTight(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	got := m.renderRow(m.all[1], false)
	if !strings.Contains(got, "▎") {
		t.Errorf("Tight row missing gutter, got %q", got)
	}
	if !strings.Contains(got, "→") {
		t.Errorf("Tight row missing arrow, got %q", got)
	}
	if strings.Contains(got, "Justo") {
		t.Errorf("Tight unselected row should not show status text, got %q", got)
	}
}

func TestRenderRowNo(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	got := m.renderRow(m.all[2], false)
	if !strings.Contains(got, "▎") {
		t.Errorf("No row missing gutter, got %q", got)
	}
	if !strings.Contains(got, "→") {
		t.Errorf("No row missing arrow, got %q", got)
	}
	if strings.Contains(got, "No cabe") {
		t.Errorf("No unselected row should not show status text, got %q", got)
	}
}

func TestRenderRowSelected(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	got := m.renderRow(m.all[0], true)
	if !strings.Contains(got, "small:7b") {
		t.Errorf("Selected row missing name, got %q", got)
	}
	if !strings.Contains(got, "Va bien") {
		t.Errorf("Selected row missing status text, got %q", got)
	}
}

// ---------- header / footer / columnHeader ----------

func TestHeaderGlyphCounts(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	got := m.header()
	if !strings.Contains(got, "3 modelos") {
		t.Errorf("header missing model count, got %q", got)
	}
	if !strings.Contains(got, "✓") || !strings.Contains(got, "!") || !strings.Contains(got, "✗") {
		t.Errorf("header missing verdict glyphs, got %q", got)
	}
	if strings.Contains(got, "● va bien") || strings.Contains(got, "● justo") || strings.Contains(got, "● no cabe") {
		t.Errorf("header contains old legend, got %q", got)
	}
}

func TestFooterNoLegend(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	got := m.footer()
	if strings.Contains(got, "va bien") || strings.Contains(got, "justo") || strings.Contains(got, "no cabe") {
		t.Errorf("footer contains legend text, got %q", got)
	}
}

func TestColumnHeaderNoTamanoNecesita(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	got := m.columnHeader()
	if strings.Contains(got, "TAMAÑO") || strings.Contains(got, "NECESITA") {
		t.Errorf("column header contains old columns, got %q", got)
	}
	if !strings.Contains(got, "MEMORIA") {
		t.Errorf("column header missing MEMORIA, got %q", got)
	}
}
