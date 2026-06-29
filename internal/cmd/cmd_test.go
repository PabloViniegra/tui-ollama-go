package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/PabloViniegra/tui-ollama-go/internal/catalog"
	"github.com/PabloViniegra/tui-ollama-go/internal/eval"
	"github.com/PabloViniegra/tui-ollama-go/internal/hardware"
	"github.com/PabloViniegra/tui-ollama-go/internal/loader"
)

// -- catalog/hardware integration --

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
	time.Sleep(5 * time.Millisecond)

	_, err := catalog.Fetch(ctx, false, false, nil)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestHardwareDetectCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	info := hardware.Detect(ctx)
	if info.OS != "" {
		t.Log("Detect returned partial info on cancelled ctx")
	}
}

// -- version --

func TestPrintVersion_ContainsFields(t *testing.T) {
	out := printVersion("dev", "none", "unknown")

	for _, want := range []string{
		"ollama-fit",
		"dev",
		runtime.GOOS,
		runtime.GOARCH,
		runtime.Version(),
	} {
		if !strings.Contains(out, want) {
			t.Errorf("printVersion() = %q, missing %q", out, want)
		}
	}
}

// -- fit helpers --

func TestFindModel_Exact(t *testing.T) {
	models := catalog.Models()
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

// -- fit --json / --explain --

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
	hw := hardware.Info{
		RAMGB: 32,
		GPU:   hardware.GPU{Name: "RTX", VRAMGB: 16, Kind: hardware.GPUKindNVIDIA},
	}
	m := catalog.Model{Name: "small:7b", Family: "small", Params: "7B", Quant: "Q4_K_M", SizeGB: 4}

	out, _ := fitReport(hw, m, true, true)

	if strings.HasPrefix(strings.TrimSpace(out), "{") && strings.Contains(out, `"verdict":`) && strings.Contains(out, "Need:") {
		t.Errorf("output mezcló JSON y texto: %q", out)
	}
}

// -- parseFitFlags --

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

func TestParseFitFlags_PrintSchema(t *testing.T) {
	opts, err := parseFitFlags([]string{"--print-schema", "qwen2.5:7b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.PrintSchema {
		t.Errorf("opts.PrintSchema = false, want true")
	}
	if opts.Model != "qwen2.5:7b" {
		t.Errorf("opts.Model = %q, want qwen2.5:7b", opts.Model)
	}
}

func TestParseFitFlags_PrintSchemaVsJSON(t *testing.T) {
	_, err := parseFitFlags([]string{"--print-schema", "--json", "x"})
	if err == nil {
		t.Error("expected error when combining --print-schema with --json")
	}
}

func TestParseFitFlags_PrintSchemaVsExplain(t *testing.T) {
	_, err := parseFitFlags([]string{"--print-schema", "--explain", "x"})
	if err == nil {
		t.Error("expected error when combining --print-schema with --explain")
	}
}

// -- doctor --

type doctorFakeRunner struct {
	output map[string]string
	err    map[string]error
}

func (f *doctorFakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	key := name + " " + strings.Join(args, " ")
	if e, ok := f.err[key]; ok {
		return "", e
	}
	if out, ok := f.output[key]; ok {
		return out, nil
	}
	return "", fmt.Errorf("command not found: %s", key)
}

func TestRunDoctor_AllPresent(t *testing.T) {
	r := &doctorFakeRunner{
		output: map[string]string{
			"nvidia-smi --version":               "NVIDIA-SMI 535.86.10",
			"ollama --version":                   "ollama version 0.5.7",
			"sysctl -n machdep.cpu.brand_string": "Apple M3 Pro",
			"uname -mrs":                         "Linux 6.5.0 x86_64",
			"rocm-smi --version":                 "ROCm 6.0.0",
			"sw_vers -productVersion":            "14.0",
		},
	}

	if got := runDoctor(r); got != 0 {
		t.Errorf("runDoctor(all-present) = %d, want 0", got)
	}
}

func TestRunDoctor_OneMissing(t *testing.T) {
	r := &doctorFakeRunner{
		output: map[string]string{
			"ollama --version":                   "ollama version 0.5.7",
			"sysctl -n machdep.cpu.brand_string": "Apple M3 Pro",
			"uname -mrs":                         "Linux 6.5.0 x86_64",
			"sw_vers -productVersion":            "14.0",
		},
	}

	if got := runDoctor(r); got != 1 {
		t.Errorf("runDoctor(one-missing) = %d, want 1", got)
	}
}

// -- local --

type localFakeRunner struct {
	output map[string]string
	err    map[string]error
}

func (f *localFakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	key := name + " " + strings.Join(args, " ")
	if e, ok := f.err[key]; ok {
		return "", e
	}
	if out, ok := f.output[key]; ok {
		return out, nil
	}
	return "", errors.New("command not found: " + key)
}

func TestLocalReport_OneLocal_Verdict(t *testing.T) {
	r := &localFakeRunner{
		output: map[string]string{
			"ollama list": "NAME                     ID              SIZE      MODIFIED\n" +
				"qwen2.5:7b              dae161e27b0e    4.7 GB    6 months ago\n",
		},
	}
	hw := hardware.Info{RAMGB: 32}
	models := catalog.Models()

	out, err := localReport(context.Background(), r, hw, models)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"qwen2.5:7b", "4.7 GB", "VERDICT"} {
		if !strings.Contains(out, want) {
			t.Errorf("localReport() missing %q\n---\n%s", want, out)
		}
	}
}

func TestLocalReport_EmptyList(t *testing.T) {
	r := &localFakeRunner{
		output: map[string]string{
			"ollama list": "NAME                     ID              SIZE      MODIFIED\n",
		},
	}

	out, err := localReport(context.Background(), r, hardware.Info{RAMGB: 32}, catalog.Models())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(strings.ToLower(out), "no ") && !strings.Contains(strings.ToLower(out), "ning") {
		t.Errorf("expected empty-list message, got %q", out)
	}
}

func TestLocalReport_RunnerError(t *testing.T) {
	r := &localFakeRunner{
		err: map[string]error{"ollama list": errors.New("ollama not in PATH")},
	}

	_, err := localReport(context.Background(), r, hardware.Info{RAMGB: 32}, nil)
	if err == nil {
		t.Fatal("expected error from runner, got nil")
	}
}

// -- runFit / runLocal with injected loader --

func fixedLoader(ramGB float64, models []catalog.Model) *loader.Source {
	return &loader.Source{
		Detect: func(_ context.Context) hardware.Info {
			return hardware.Info{RAMGB: ramGB}
		},
		Fetch: func(_ context.Context, _, _ bool, _ func(string)) ([]catalog.Model, error) {
			return models, nil
		},
	}
}

func TestRunFit_HappyPath(t *testing.T) {
	src := fixedLoader(32, []catalog.Model{
		{Name: "qwen2.5:7b", Family: "qwen2.5", Params: "7B", Quant: "Q4_K_M", SizeGB: 4.7},
	})

	if code := runFit([]string{"qwen2.5:7b"}, src, nil); code != 0 {
		t.Errorf("exit code = %d, want 0 (Good)", code)
	}
}

func TestRunFit_NotFound(t *testing.T) {
	src := fixedLoader(32, catalog.Models())

	if code := runFit([]string{"no-existe-99b"}, src, nil); code != 3 {
		t.Errorf("exit code = %d, want 3 (model not found)", code)
	}
}

func TestRunFit_NoFit(t *testing.T) {
	src := fixedLoader(16, []catalog.Model{
		{Name: "huge:70b", Family: "huge", Params: "70B", Quant: "Q4_K_M", SizeGB: 70},
	})

	if code := runFit([]string{"huge:70b"}, src, nil); code != 2 {
		t.Errorf("exit code = %d, want 2 (No)", code)
	}
}

func TestRunFit_PrintSchema(t *testing.T) {
	cases := [][]string{
		{"--print-schema"},
		{"--print-schema", "ignored-model"},
	}
	for _, args := range cases {
		if code := runFit(args, fixedLoader(0, nil), []byte("{}")); code != 0 {
			t.Errorf("runFit(%v) = %d, want 0", args, code)
		}
	}
}

func TestRunLocal_EmptyArgs_OK(t *testing.T) {
	src := fixedLoader(32, []catalog.Model{
		{Name: "qwen2.5:7b", Family: "qwen2.5", Params: "7B", Quant: "Q4_K_M", SizeGB: 4.7},
	})
	runner := &localFakeRunner{
		output: map[string]string{
			"ollama list": "NAME       SIZE\nqwen2.5:7b 4.7 GB\n",
		},
	}

	if code := runLocal([]string{}, src, runner); code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRunLocal_RejectsArgs(t *testing.T) {
	src := fixedLoader(32, nil)
	runner := &localFakeRunner{}

	if code := runLocal([]string{"foo"}, src, runner); code != 3 {
		t.Errorf("exit code = %d, want 3 (args inválidos)", code)
	}
}

func TestRunLocal_RunnerError(t *testing.T) {
	src := fixedLoader(32, catalog.Models())
	runner := &localFakeRunner{
		err: map[string]error{"ollama list": errors.New("ollama not in PATH")},
	}

	if code := runLocal([]string{}, src, runner); code != 3 {
		t.Errorf("exit code = %d, want 3 (runner falló)", code)
	}
}
