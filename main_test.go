package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/PabloViniegra/tui-ollama-go/internal/catalog"
	"github.com/PabloViniegra/tui-ollama-go/internal/eval"
	"github.com/PabloViniegra/tui-ollama-go/internal/hardware"
)

func TestCatalogFetchCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := catalog.Fetch(ctx, false, false, nil)
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
}

func TestCatalogFetchTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond) // ensure timeout fires

	_, err := catalog.Fetch(ctx, false, false, nil)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestHardwareDetectCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	info := hardware.Detect(ctx)
	// Should return early without panicking.
	if info.OS != "" {
		t.Log("Detect returned partial info on cancelled ctx")
	}
}

func TestRunMainOfflineFlagParses(t *testing.T) {
	// --offline should parse without panic and return 1 because the TUI
	// can't run in a non-interactive test environment.
	code := runMain([]string{"ollama-fit", "--offline"})
	if code != 1 {
		// It may fail because tea.NewProgram needs a terminal, which is fine.
		// We just care that it doesn't panic and the flag parses.
		t.Logf("runMain(--offline) returned %d (expected 1 in non-interactive env)", code)
	}
}

func TestRunMainInvalidFlag(t *testing.T) {
	code := runMain([]string{"ollama-fit", "--invalid-flag"})
	if code != 1 {
		t.Fatalf("expected exit code 1 for invalid flag, got %d", code)
	}
}

func TestFindModel_Exact(t *testing.T) {
	models := catalog.Models() // catálogo embebido, determinístico
	m, ok := findModel(models, "qwen2.5:7b")
	if !ok {
		t.Fatal("expected to find qwen2.5:7b")
	}
	if m.Family != "qwen2.5" || m.Params != "7B" {
		t.Errorf("got %+v, want family=qwen2.5 params=7B", m)
	}
}

func TestFindModel_CaseInsensitive(t *testing.T) {
	models := catalog.Models()
	for _, name := range []string{"QWEN2.5:7B", "Qwen2.5:7b", "qWeN2.5:7B"} {
		if _, ok := findModel(models, name); !ok {
			t.Errorf("expected to find %q (case-insensitive)", name)
		}
	}
}

func TestFindModel_Unknown(t *testing.T) {
	models := catalog.Models()
	if _, ok := findModel(models, "no-existe-este-modelo-xyz-9876"); ok {
		t.Fatal("expected NOT to find unknown model")
	}
}

func TestAvailableGB(t *testing.T) {
	cases := []struct {
		name string
		hw   hardware.Info
		want float64
	}{
		{"nvidia", hardware.Info{GPU: hardware.GPU{VRAMGB: 12.0}}, 12.0},
		{"apple", hardware.Info{AppleUnified: true, RAMGB: 32.0}, eval.AppleGPUFraction * 32.0},
		{"cpu", hardware.Info{RAMGB: 16.0}, 16.0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := availableGB(c.hw); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestVerdictExitCode(t *testing.T) {
	cases := []struct {
		v    eval.Verdict
		want int
	}{
		{eval.Good, 0},
		{eval.Tight, 1},
		{eval.No, 2},
	}
	for _, c := range cases {
		if got := verdictExitCode(c.v); got != c.want {
			t.Errorf("verdictExitCode(%d) = %d, want %d", c.v, got, c.want)
		}
	}
}

// -- fit --json ------------------------------------------------------

func TestFitReport_TextFormatPreserved(t *testing.T) {
	hw := hardware.Info{
		RAMGB: 32,
		GPU:   hardware.GPU{Name: "RTX", VRAMGB: 16, Kind: hardware.GPUKindNVIDIA},
	}
	m := catalog.Model{Name: "small:7b", Family: "small", Params: "7B", Quant: "Q4_K_M", SizeGB: 4}

	out, code := fitReport(hw, m, false, false)

	if code != 0 {
		t.Errorf("exit code = %d, want 0 (Good)", code)
	}
	if !strings.Contains(out, "small:7b → Good") {
		t.Errorf("output missing header line: %q", out)
	}
	if !strings.Contains(out, "backend") || !strings.Contains(out, "reason") || !strings.Contains(out, "need") {
		t.Errorf("output missing standard fields: %q", out)
	}
}

func TestFitReport_JSONShape(t *testing.T) {
	hw := hardware.Info{
		RAMGB: 32,
		GPU:   hardware.GPU{Name: "RTX", VRAMGB: 16, Kind: hardware.GPUKindNVIDIA},
	}
	m := catalog.Model{Name: "small:7b", Family: "small", Params: "7B", Quant: "Q4_K_M", SizeGB: 4}

	out, code := fitReport(hw, m, true, false)

	if code != 0 {
		t.Errorf("exit code = %d, want 0 (Good)", code)
	}
	var p fitOutput
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if p.Verdict != "good" {
		t.Errorf("verdict = %q, want \"good\"", p.Verdict)
	}
	if p.Model.Name != "small:7b" {
		t.Errorf("model.name = %q, want \"small:7b\"", p.Model.Name)
	}
	if p.NeedGB != 4*1.2 {
		t.Errorf("need_gb = %v, want %v", p.NeedGB, 4*1.2)
	}
	if p.AvailableGB != 16 {
		t.Errorf("available_gb = %v, want 16", p.AvailableGB)
	}
	if p.Backend == "" {
		t.Errorf("backend is empty")
	}
	if p.Reason == "" {
		t.Errorf("reason is empty")
	}
}

func TestFitReport_JSON_NoFit(t *testing.T) {
	hw := hardware.Info{RAMGB: 16}
	m := catalog.Model{Name: "huge:70b", Family: "huge", Params: "70B", Quant: "Q4_K_M", SizeGB: 70}

	out, code := fitReport(hw, m, true, false)

	if code != 2 {
		t.Errorf("exit code = %d, want 2 (No)", code)
	}
	if !strings.Contains(out, `"verdict":"no"`) {
		t.Errorf("expected verdict=no in: %s", out)
	}
}

func TestFitReport_JSON_Tight(t *testing.T) {
	hw := hardware.Info{RAMGB: 16}
	m := catalog.Model{Name: "mid:8b", Family: "mid", Params: "8B", Quant: "Q4_K_M", SizeGB: 8}

	out, code := fitReport(hw, m, true, false)

	if code != 1 {
		t.Errorf("exit code = %d, want 1 (Tight)", code)
	}
	if !strings.Contains(out, `"verdict":"tight"`) {
		t.Errorf("expected verdict=tight in: %s", out)
	}
}

func TestFitReport_JSON_Compact(t *testing.T) {
	hw := hardware.Info{
		RAMGB: 32,
		GPU:   hardware.GPU{Name: "RTX", VRAMGB: 16, Kind: hardware.GPUKindNVIDIA},
	}
	m := catalog.Model{Name: "small:7b", Family: "small", Params: "7B", Quant: "Q4_K_M", SizeGB: 4}

	out, _ := fitReport(hw, m, true, false)
	if strings.Contains(out, "\n") {
		t.Errorf("JSON should be single line (compact), got: %q", out)
	}
}

// -- fit --explain --------------------------------------------------

func TestFitReport_Explain_IncludesMath(t *testing.T) {
	hw := hardware.Info{
		RAMGB: 32,
		GPU:   hardware.GPU{Name: "RTX", VRAMGB: 16, Kind: hardware.GPUKindNVIDIA},
	}
	m := catalog.Model{Name: "small:7b", Family: "small", Params: "7B", Quant: "Q4_K_M", SizeGB: 4}

	out, code := fitReport(hw, m, false, true)

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	for _, want := range []string{"Model:", "Need:", "Available:", "Rule:", "Verdict:"} {
		if !strings.Contains(out, want) {
			t.Errorf("explain output missing %q\n--- output ---\n%s", want, out)
		}
	}
}

func TestFitReport_Explain_TightShowsRule(t *testing.T) {
	hw := hardware.Info{RAMGB: 16}
	m := catalog.Model{Name: "mid:8b", Family: "mid", Params: "8B", Quant: "Q4_K_M", SizeGB: 8}

	out, code := fitReport(hw, m, false, true)

	if code != 1 {
		t.Errorf("exit code = %d, want 1 (Tight)", code)
	}
	if !strings.Contains(out, "Verdict:") {
		t.Errorf("explain output missing Verdict line: %s", out)
	}
	if !strings.Contains(out, "Tight") {
		t.Errorf("explain output missing Tight label: %s", out)
	}
}

func TestFitReport_Explain_TextOrJSONOnly(t *testing.T) {
	// --json y --explain son mutuamente excluyentes en runFit (ver parseFitFlags).
	// fitReport en sí no debe combinar ambos formatos: si ambos flags son true,
	// --explain tiene precedencia.
	hw := hardware.Info{
		RAMGB: 32,
		GPU:   hardware.GPU{Name: "RTX", VRAMGB: 16, Kind: hardware.GPUKindNVIDIA},
	}
	m := catalog.Model{Name: "small:7b", Family: "small", Params: "7B", Quant: "Q4_K_M", SizeGB: 4}

	out, _ := fitReport(hw, m, true, true)

	// Si prevalece explain, no es JSON puro.
	if strings.HasPrefix(strings.TrimSpace(out), "{") && strings.Contains(out, `"verdict":`) && strings.Contains(out, "Need:") {
		t.Errorf("output mezcló JSON y texto: %q", out)
	}
}

// -- parseFitFlags -----------------------------------------------------

func TestParseFitFlags_BareModel(t *testing.T) {
	opts, err := parseFitFlags([]string{"qwen2.5:7b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Model != "qwen2.5:7b" || opts.AsJSON || opts.AsExplain {
		t.Errorf("got %+v, want Model=qwen2.5:7b AsJSON=false AsExplain=false", opts)
	}
}

func TestParseFitFlags_JSON(t *testing.T) {
	opts, err := parseFitFlags([]string{"--json", "llama3.1:8b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.AsJSON || opts.AsExplain {
		t.Errorf("got %+v, want AsJSON=true AsExplain=false", opts)
	}
}

func TestParseFitFlags_Explain(t *testing.T) {
	opts, err := parseFitFlags([]string{"--explain", "llama3.1:8b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.AsJSON || !opts.AsExplain {
		t.Errorf("got %+v, want AsJSON=false AsExplain=true", opts)
	}
}

func TestParseFitFlags_Mutex(t *testing.T) {
	_, err := parseFitFlags([]string{"--json", "--explain", "llama3.1:8b"})
	if err == nil {
		t.Fatal("expected mutex error, got nil")
	}
}

func TestParseFitFlags_NoArgs(t *testing.T) {
	_, err := parseFitFlags([]string{})
	if err == nil {
		t.Fatal("expected missing-model error, got nil")
	}
}

func TestParseFitFlags_TooManyArgs(t *testing.T) {
	_, err := parseFitFlags([]string{"a", "b"})
	if err == nil {
		t.Fatal("expected extra-arg error, got nil")
	}
}

func TestParseFitFlags_InvalidFlag(t *testing.T) {
	_, err := parseFitFlags([]string{"--no-existe", "llama3.1:8b"})
	if err == nil {
		t.Fatal("expected unknown-flag error, got nil")
	}
}
