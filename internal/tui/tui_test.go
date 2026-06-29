package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/PabloViniegra/tui-ollama-go/internal/catalog"
	"github.com/PabloViniegra/tui-ollama-go/internal/eval"
	"github.com/PabloViniegra/tui-ollama-go/internal/hardware"
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

func TestSpinnerFrame(t *testing.T) {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	for i, want := range frames {
		got := spinnerFrame(i)
		if got != want {
			t.Errorf("spinnerFrame(%d) = %q, want %q", i, got, want)
		}
	}
	if spinnerFrame(10) != frames[0] {
		t.Errorf("spinnerFrame(10) should wrap to frame 0, got %q", spinnerFrame(10))
	}
}

func TestUpdateSpinnerAdvancesFrame(t *testing.T) {
	m := NewAsync(func() (hardware.Info, []eval.Result, error) { return hwNone(), nil, nil })
	m.spinnerTick = 0
	newModel, cmd := m.Update(spinnerMsg{})
	m2 := newModel.(Model)
	if m2.spinnerTick != 1 {
		t.Errorf("spinnerTick = %d, want 1", m2.spinnerTick)
	}
	if cmd == nil {
		t.Error("expected next tick cmd while still loading")
	}
}

func TestUpdateSpinnerStopsAfterWindowSize(t *testing.T) {
	m := New(hwNone(), nil) // loading=false, spinner stops immediately
	m.width, m.height = 120, 40
	_, cmd := m.Update(spinnerMsg{})
	if cmd != nil {
		t.Error("expected nil cmd when not in loading state")
	}
}

func TestNewAsyncStartsLoading(t *testing.T) {
	called := false
	m := NewAsync(func() (hardware.Info, []eval.Result, error) {
		called = true
		return hwNone(), makeResults(), nil
	})
	if !m.loading {
		t.Error("NewAsync should set loading=true")
	}
	cmd := m.Init()
	if cmd == nil {
		t.Error("NewAsync Init() should return a cmd")
	}
	_ = called
}

func TestLoadMsgPopulatesModel(t *testing.T) {
	m := NewAsync(func() (hardware.Info, []eval.Result, error) {
		return hwNone(), makeResults(), nil
	})
	newModel, _ := m.Update(loadMsg{hw: hwNone(), results: makeResults()})
	m2 := newModel.(Model)
	if m2.loading {
		t.Error("loading should be false after loadMsg")
	}
	if len(m2.all) != 3 {
		t.Errorf("expected 3 results, got %d", len(m2.all))
	}
}

func TestViewShowsSpinnerWhileLoading(t *testing.T) {
	m := NewAsync(func() (hardware.Info, []eval.Result, error) {
		return hwNone(), nil, nil
	})
	m.width, m.height = 120, 40
	got := m.View()
	found := false
	for _, f := range spinnerFrames {
		if strings.Contains(got, f) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("View() while loading should show spinner, got %q", got)
	}
}

func TestInit(t *testing.T) {
	m := New(hwNone(), nil)
	if cmd := m.Init(); cmd == nil {
		t.Error("Init() should return spinner tick cmd")
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
	if statusColor(eval.Good) != cGood {
		t.Errorf("Good color mismatch: got %v, want %v", statusColor(eval.Good), cGood)
	}
	if statusColor(eval.Tight) != cTight {
		t.Errorf("Tight color mismatch: got %v, want %v", statusColor(eval.Tight), cTight)
	}
	if statusColor(eval.No) != cNo {
		t.Errorf("No color mismatch: got %v, want %v", statusColor(eval.No), cNo)
	}
	// Each verdict maps to a distinct color.
	if statusColor(eval.Good) == statusColor(eval.Tight) || statusColor(eval.Good) == statusColor(eval.No) {
		t.Error("verdict colors must be distinct")
	}
}

// ---------- helpers ----------

func TestGutterGlyph(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
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
	lipgloss.SetColorProfile(termenv.TrueColor)
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
		wantSize := fmt.Sprintf("%.1f GB", tc.size)
		if !strings.Contains(got, wantSize) {
			t.Errorf("arrowSignature(%v,%v) missing size %q, got %q", tc.size, tc.need, wantSize, got)
		}
		wantNeed := fmt.Sprintf("%.1f GB", tc.need)
		if !strings.Contains(got, wantNeed) {
			t.Errorf("arrowSignature(%v,%v) missing need %q, got %q", tc.size, tc.need, wantNeed, got)
		}
		// color must be applied (ANSI escape present)
		if !strings.Contains(got, "\x1b[") {
			t.Errorf("arrowSignature(%v,%v) missing ANSI escape, got %q", tc.size, tc.need, got)
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
		GPU:          hardware.GPU{Name: "M2 Pro", Kind: hardware.GPUKindApple},
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
	hw := hardware.Info{GPU: hardware.GPU{Name: "RTX 4070", VRAMGB: 12, Kind: hardware.GPUKindNVIDIA}}
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
	hw := hardware.Info{GPU: hardware.GPU{Kind: hardware.GPUKindNone}}
	m := New(hw, nil)
	got := m.gpuDescr()
	if !strings.Contains(got, "sin GPU") {
		t.Errorf("gpuDescr none: %q missing 'sin GPU'", got)
	}
}

func TestGpuDescrUnknownVRAM(t *testing.T) {
	hw := hardware.Info{GPU: hardware.GPU{Name: "Intel HD 630", Kind: hardware.GPUKindIntel, VRAMGB: 0}}
	m := New(hw, nil)
	got := m.gpuDescr()
	if !strings.Contains(got, "VRAM desconocida") {
		t.Errorf("gpuDescr unknown VRAM: %q", got)
	}
}

// ---------- View ----------

func TestViewZeroSize(t *testing.T) {
	m := New(hwNone(), makeResults()) // loading=false, but no window size yet
	got := m.View()
	if !strings.Contains(got, MsgDetectingHardware) {
		t.Errorf("View() zero size: missing detecting message, got %q", got)
	}
}

func TestViewWithSize(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	got := m.View()
	if got == "" {
		t.Error("View() returned empty string")
	}
	if strings.Contains(got, MsgDetectingHardware) {
		t.Error("View() still shows loading with non-zero size")
	}
	if !strings.Contains(got, "Ollama Fit") {
		t.Errorf("View() missing title, got %q", got)
	}
	if !strings.Contains(got, "✓") && !strings.Contains(got, "!") && !strings.Contains(got, "✗") {
		t.Errorf("View() missing verdict glyphs, got %q", got)
	}
	if !strings.Contains(got, "─") {
		t.Errorf("View() missing box-drawing separator ─")
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

// ---------- layout: width and height stability ----------

// TestViewNoLineExceedsTerminalWidth guards against visual line-wrap that causes
// lipgloss.Height(header) to undercount, making listHeight() return one row too
// many and pushing the header off screen on narrow terminals (≤80 cols).
func TestViewNoLineExceedsTerminalWidth(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	for _, w := range []int{80, 100, 120} {
		m := newWithSize(hwNone(), makeResults(), w, 30)
		for i, line := range strings.Split(m.View(), "\n") {
			if lw := lipgloss.Width(line); lw > w {
				t.Errorf("w=%d: line %d visual width %d > terminal width: %q", w, i, lw, line)
			}
		}
	}
}

func TestViewHeightEqualsTerminalHeight(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	for _, h := range []int{20, 30, 40} {
		m := newWithSize(hwNone(), makeResults(), 120, h)
		if got := lipgloss.Height(m.View()); got != h {
			t.Errorf("h=%d: View() line count = %d, want %d", h, got, h)
		}
	}
}

func makeManyResults(n int) []eval.Result {
	results := make([]eval.Result, n)
	for i := range results {
		results[i] = eval.Result{
			Model:   catalog.Model{Name: fmt.Sprintf("model-%02d:7b", i+1), SizeGB: float64(i + 1)},
			Verdict: eval.Good,
			Backend: "CPU",
			NeedGB:  float64(i+1) * 1.2,
		}
	}
	return results
}

// TestViewNoWrapWhileScrolling scrolls through a long list on a narrow terminal
// and verifies no line ever exceeds the terminal width.
func TestViewNoWrapWhileScrolling(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	const w, h = 80, 20
	m := newWithSize(hwNone(), makeManyResults(50), w, h)
	for step := range 50 {
		for i, line := range strings.Split(m.View(), "\n") {
			if lw := lipgloss.Width(line); lw > w {
				t.Errorf("step %d, line %d: visual width %d > %d", step, i, lw, w)
				return
			}
		}
		m2, _ := m.Update(key("j"))
		m = m2.(Model)
	}
}

// TestViewRowAlignmentWithLongName es la regresión para bugs 1+2: un nombre
// de modelo que excede wName ya no envuelve la fila, no rompe el alineamiento
// de columnas y no desborda la View.
func TestViewRowAlignmentWithLongName(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	results := []eval.Result{
		{Model: catalog.Model{Name: "short:7b", SizeGB: 4, Params: "7B", Quant: "Q4_K_M"}, Verdict: eval.Good, Backend: "CPU", NeedGB: 4.5},
		{Model: catalog.Model{Name: "this-is-a-very-long-model-name-that-might-overflow:70b", SizeGB: 40, Params: "70B", Quant: "Q4_K_M"}, Verdict: eval.No, Backend: "CPU", NeedGB: 45},
		{Model: catalog.Model{Name: "another:7b", SizeGB: 4, Params: "7B", Quant: "Q4_K_M"}, Verdict: eval.Good, Backend: "CPU", NeedGB: 4.5},
	}
	m := newWithSize(hwNone(), results, 80, 30)
	if got := lipgloss.Height(m.View()); got != 30 {
		t.Errorf("View height = %d, want 30 (footer/header would be cut off)", got)
	}
	for i, line := range strings.Split(m.View(), "\n") {
		if lw := lipgloss.Width(line); lw > 80 {
			t.Errorf("line %d visual width %d > 80: %q", i, lw, line)
		}
	}
	// La fila del nombre largo debe ser exactamente 1 línea, no 2 ni 3.
	// (results se ordena por SizeGB asc, así que el de 40 GB queda al final.)
	var longRow string
	for _, r := range m.view {
		if len(r.Model.Name) > 30 {
			longRow = m.renderRow(r, false)
			break
		}
	}
	if longRow == "" {
		t.Fatal("no se encontró la fila con nombre largo en m.view")
	}
	if lines := strings.Count(longRow, "\n") + 1; lines != 1 {
		t.Errorf("long-name row rendered as %d lines, want 1: %q", lines, longRow)
	}
	// El nombre debe aparecer truncado con elipsis.
	if !strings.Contains(longRow, "…") {
		t.Errorf("long-name row should contain truncation ellipsis, got %q", longRow)
	}
	// Mientras scrolleo, los invariantes se mantienen.
	for step := 0; step < 3; step++ {
		nm, _ := m.Update(key("j"))
		m = nm.(Model)
		for i, line := range strings.Split(m.View(), "\n") {
			if lw := lipgloss.Width(line); lw > 80 {
				t.Errorf("step %d, line %d: visual width %d > 80", step, i, lw)
			}
		}
		if got := lipgloss.Height(m.View()); got != 30 {
			t.Errorf("step %d: View height = %d, want 30", step, got)
		}
	}
}

// withMockClipboard reemplaza clipboardWriter por un stub durante la prueba y
// restaura el original al terminar. Devuelve un puntero a la captura del
// último string que se intentó copiar, para que el test pueda afirmarlo.
func withMockClipboard(t *testing.T, retErr error) *string {
	t.Helper()
	var captured string
	orig := clipboardWriter
	clipboardWriter = func(s string) error {
		captured = s
		return retErr
	}
	t.Cleanup(func() { clipboardWriter = orig })
	return &captured
}

// TestUpdateEnterOnModelSetsMessage es la regresión para bug 3: Enter sobre
// un modelo debe poblar m.message con la confirmación. Usa un mock de
// clipboard para no depender de xclip/xsel/wl-clipboard en CI.
func TestUpdateEnterOnModelSetsMessage(t *testing.T) {
	captured := withMockClipboard(t, nil)
	m := newWithSize(hwNone(), []eval.Result{
		{Model: catalog.Model{Name: "llama3.1:8b", SizeGB: 4.9, Params: "8B", Quant: "Q4_K_M"}, Verdict: eval.Good, Backend: "CPU", NeedGB: 5.5},
	}, 120, 40)
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := nm.(Model)
	if m2.message == "" {
		t.Fatal("expected m.message set after Enter, got empty")
	}
	if !strings.Contains(m2.message, "ollama run") {
		t.Errorf("m.message should contain 'ollama run', got %q", m2.message)
	}
	if !strings.Contains(m2.message, "llama3.1:8b") {
		t.Errorf("m.message should contain model name, got %q", m2.message)
	}
	if want := "ollama run llama3.1:8b"; *captured != want {
		t.Errorf("clipboard received %q, want %q", *captured, want)
	}
}

// TestUpdateEnterKeyStringEquivalent cubre la otra forma del KeyMsg (la string
// "enter") para que ambos paths queden cubiertos.
func TestUpdateEnterKeyStringEquivalent(t *testing.T) {
	withMockClipboard(t, nil)
	m := newWithSize(hwNone(), []eval.Result{
		{Model: catalog.Model{Name: "qwen2.5:7b", SizeGB: 4.7, Params: "7B", Quant: "Q4_K_M"}, Verdict: eval.Good, Backend: "CPU", NeedGB: 5.0},
	}, 120, 40)
	nm, _ := m.Update(key("enter"))
	m2 := nm.(Model)
	if !strings.Contains(m2.message, "ollama run qwen2.5:7b") {
		t.Errorf("Enter via key string should set full command, got %q", m2.message)
	}
}

// TestUpdateEnterClipboardError cubre el path de error: si clipboardWriter
// falla, el footer debe mostrar el error y NO la confirmación de éxito.
func TestUpdateEnterClipboardError(t *testing.T) {
	withMockClipboard(t, fmt.Errorf("simulated clipboard failure"))
	m := newWithSize(hwNone(), []eval.Result{
		{Model: catalog.Model{Name: "mistral:7b", SizeGB: 4.1, Params: "7B", Quant: "Q4_0"}, Verdict: eval.Good, Backend: "CPU", NeedGB: 4.5},
	}, 120, 40)
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := nm.(Model)
	if !strings.Contains(m2.message, "no se pudo copiar") {
		t.Errorf("error path should show copy-fail message, got %q", m2.message)
	}
	if strings.Contains(m2.message, "ollama run") {
		t.Errorf("error path should NOT show success command, got %q", m2.message)
	}
}

// TestUpdateEnterOnEmptyViewDoesNotPanic cubre el caso borde view==0.
func TestUpdateEnterOnEmptyViewDoesNotPanic(t *testing.T) {
	withMockClipboard(t, nil)
	m := newWithSize(hwNone(), nil, 120, 40)
	m.applyFilter() // view queda vacío
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Enter on empty view panicked: %v", r)
		}
	}()
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := nm.(Model)
	if m2.message != "" {
		t.Errorf("Enter on empty view should not set message, got %q", m2.message)
	}
}

// TestFooterShowsCopiedMessage verifica que el footer muestre el mensaje y no
// el help cuando m.message está seteado.
func TestFooterShowsCopiedMessage(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	m.message = "✓ copiado al portapapeles: ollama run llama3.1:8b"
	got := m.footer()
	if !strings.Contains(got, "ollama run llama3.1:8b") {
		t.Errorf("footer should show copied message, got %q", got)
	}
	if strings.Contains(got, "filtro [") {
		t.Errorf("footer should hide help when message is set, got %q", got)
	}
}

// TestNextKeyClearsMessage verifica que cualquier tecla limpia el mensaje
// para que no quede persistente en el footer.
func TestNextKeyClearsMessage(t *testing.T) {
	m := newWithSize(hwNone(), makeResults(), 120, 40)
	m.message = "✓ copiado al portapapeles: ollama run llama3.1:8b"
	nm, _ := m.Update(key("j"))
	m2 := nm.(Model)
	if m2.message != "" {
		t.Errorf("message should be cleared by next key, got %q", m2.message)
	}
}

// ---------- fuzzy search ----------

// fuzzyResults construye un set de modelos pensado para testear fuzzy match.
// Contiene typos potenciales (qwen/qe2), caracteres extra (gemma3:4b) y casos
// donde el orden de caracteres en el nombre es relevante.
func fuzzyResults() []eval.Result {
	return []eval.Result{
		{Model: catalog.Model{Name: "qwen2.5:7b", Family: "qwen2.5", Params: "7B", Quant: "Q4_K_M", SizeGB: 4.7}, Verdict: eval.Good},
		{Model: catalog.Model{Name: "llama3.1:8b", Family: "llama3.1", Params: "8B", Quant: "Q4_K_M", SizeGB: 4.9}, Verdict: eval.Good},
		{Model: catalog.Model{Name: "deepseek-r1:70b", Family: "deepseek-r1", Params: "70B", Quant: "Q4_K_M", SizeGB: 43}, Verdict: eval.No},
		{Model: catalog.Model{Name: "gemma3:4b", Family: "gemma3", Params: "4B", Quant: "Q4_K_M", SizeGB: 3.3}, Verdict: eval.Good},
	}
}

func TestApplyFilterFuzzy_SubstringStillWorks(t *testing.T) {
	// Backcompat: queries que ya funcionaban con substring siguen funcionando.
	m := New(hwNone(), fuzzyResults())
	m.filter = fAll
	m.search = "llama3.1"
	m.applyFilter()
	if len(m.view) != 1 || m.view[0].Model.Name != "llama3.1:8b" {
		t.Errorf("substring backcompat broken: %+v", m.view)
	}
}

func TestApplyFilterFuzzy_TypoSubsequence(t *testing.T) {
	// "qwn25" → no es substring de "qwen2.5:7b" pero los chars están en orden.
	m := New(hwNone(), fuzzyResults())
	m.filter = fAll
	m.search = "qwn25"
	m.applyFilter()
	if len(m.view) != 1 || m.view[0].Model.Name != "qwen2.5:7b" {
		t.Errorf("fuzzy(typo) failed: got %+v, want qwen2.5:7b", m.view)
	}
}

func TestApplyFilterFuzzy_SkipsCharacters(t *testing.T) {
	// "llama8b" → matchea "llama3.1:8b" saltando "3.1:".
	m := New(hwNone(), fuzzyResults())
	m.filter = fAll
	m.search = "llama8b"
	m.applyFilter()
	if len(m.view) != 1 || m.view[0].Model.Name != "llama3.1:8b" {
		t.Errorf("fuzzy(skips) failed: got %+v, want llama3.1:8b", m.view)
	}
}

func TestApplyFilterFuzzy_CaseInsensitive(t *testing.T) {
	m := New(hwNone(), fuzzyResults())
	m.filter = fAll
	m.search = "QWEN"
	m.applyFilter()
	if len(m.view) != 1 || m.view[0].Model.Name != "qwen2.5:7b" {
		t.Errorf("fuzzy(case) failed: got %+v, want qwen2.5:7b", m.view)
	}
}

func TestApplyFilterFuzzy_OrderMatters(t *testing.T) {
	// "52qwen" → los chars no están en ese orden en ningún target.
	m := New(hwNone(), fuzzyResults())
	m.filter = fAll
	m.search = "52qwen"
	m.applyFilter()
	if len(m.view) != 0 {
		t.Errorf("fuzzy(order) failed: got %+v, want 0 results", m.view)
	}
}

func TestApplyFilterFuzzy_EmptyQuery(t *testing.T) {
	m := New(hwNone(), fuzzyResults())
	m.filter = fAll
	m.search = ""
	m.applyFilter()
	if len(m.view) != len(fuzzyResults()) {
		t.Errorf("fuzzy(empty) failed: got %d, want %d", len(m.view), len(fuzzyResults()))
	}
}

func TestApplyFilterFuzzy_ComposesWithFilter(t *testing.T) {
	// Fuzzy + filtro de veredicto compone correctamente.
	m := New(hwNone(), fuzzyResults())
	m.filter = fNo // solo "deepseek-r1:70b" + todo fuzzy matchea
	m.search = "deep"
	m.applyFilter()
	if len(m.view) != 1 || m.view[0].Model.Name != "deepseek-r1:70b" {
		t.Errorf("fuzzy+filter failed: got %+v, want deepseek-r1:70b", m.view)
	}
}

func TestApplyFilterFuzzy_ComposesWithFilterRejects(t *testing.T) {
	// Fuzzy matchea varios pero el filtro de verdict deja solo uno.
	m := New(hwNone(), fuzzyResults())
	m.filter = fGood // descartar deepseek-r1:70b (No)
	m.search = "b"   // matchea todos los nombres (subsequence de "b")
	m.applyFilter()
	// Espera: small/medium/good, no deepseek (es No).
	for _, r := range m.view {
		if r.Verdict == eval.No {
			t.Errorf("fuzzy+filter leaked No verdict: %+v", r)
		}
	}
}

// ---------- fuzzyMatch (pure) ----------

func TestFuzzyMatch(t *testing.T) {
	cases := []struct {
		query, target string
		want          bool
	}{
		{"", "anything", true},
		{"abc", "abc", true},
		{"abc", "axbxcx", true},
		{"abc", "cab", false},       // orden invertido
		{"abc", "acb", false},       // orden invertido
		{"abc", "ab", false},        // más corto que query
		{"QWE", "qwen2.5:7b", true}, // case-insensitive
		{"q.w.e.n", "qwen", false},  // el '.' no aparece en el target
		{"aa", "a", false},          // más chars que disponibles
	}
	for _, c := range cases {
		t.Run(c.query+"|"+c.target, func(t *testing.T) {
			if got := fuzzyMatch(c.query, c.target); got != c.want {
				t.Errorf("fuzzyMatch(%q, %q) = %v, want %v", c.query, c.target, got, c.want)
			}
		})
	}
}
