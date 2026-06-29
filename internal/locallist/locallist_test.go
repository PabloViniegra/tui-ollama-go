package locallist

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/PabloViniegra/tui-ollama-go/internal/catalog"
	"github.com/PabloViniegra/tui-ollama-go/internal/eval"
	"github.com/PabloViniegra/tui-ollama-go/internal/hardware"
)

// fakeRunner implements CommandRunner for tests.
type fakeRunner struct {
	output map[string]string
	err    map[string]error
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	key := name + " " + strings.Join(args, " ")
	if e, ok := f.err[key]; ok {
		return "", e
	}
	if out, ok := f.output[key]; ok {
		return out, nil
	}
	return "", fmt.Errorf("command not found: %s", key)
}

// -- ParseOllamaList --------------------------------------------------

func TestParseOllamaList_Typical(t *testing.T) {
	out := `NAME                     ID              SIZE      MODIFIED
gemma4:31b-cloud         c382fbfbc73b    -         2 months ago
qwen2.5-coder:7b         dae161e27b0e    4.7 GB    6 months ago
deepseek-coder-v2:16b    63fb193b3a9b    8.9 GB    6 months ago
`

	got := ParseOllamaList(out)

	if len(got) != 3 {
		t.Fatalf("ParseOllamaList() len = %d, want 3; got %+v", len(got), got)
	}
	want := []LocalEntry{
		{Name: "gemma4:31b-cloud", Size: "-"},
		{Name: "qwen2.5-coder:7b", Size: "4.7 GB"},
		{Name: "deepseek-coder-v2:16b", Size: "8.9 GB"},
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("[%d] got %+v, want %+v", i, got[i], w)
		}
	}
}

func TestParseOllamaList_Empty(t *testing.T) {
	if got := ParseOllamaList(""); len(got) != 0 {
		t.Errorf("ParseOllamaList(empty) = %+v, want empty", got)
	}
}

func TestParseOllamaList_SkipsShortLines(t *testing.T) {
	out := `NAME                     ID              SIZE      MODIFIED
random-garbage-line
qwen2.5-coder:7b         dae161e27b0e    4.7 GB    6 months ago
`
	got := ParseOllamaList(out)
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1 (header + 2 lines, 1 too short); got %+v", len(got), got)
	}
	if got[0].Name != "qwen2.5-coder:7b" {
		t.Errorf("Name = %q, want qwen2.5-coder:7b", got[0].Name)
	}
}

func TestParseOllamaList_ExtraSpacesAndTabs(t *testing.T) {
	// El formato real de `ollama list` alinea columnas con 2+ espacios (o tabs).
	out := "NAME              ID              SIZE      MODIFIED\n" +
		"qwen3:8b         abc123          5.1 GB    yesterday\n"
	got := ParseOllamaList(out)
	if len(got) != 1 || got[0].Name != "qwen3:8b" || got[0].Size != "5.1 GB" {
		t.Errorf("got %+v, want [{qwen3:8b 5.1 GB}]", got)
	}
}

// -- EvaluateLocal ----------------------------------------------------

func TestEvaluateLocal_FindsCatalogMatches(t *testing.T) {
	r := &fakeRunner{
		output: map[string]string{
			"ollama list": "NAME                     ID              SIZE      MODIFIED\n" +
				"qwen2.5:7b              dae161e27b0e    4.7 GB    6 months ago\n" +
				"no-existe:99b           dae161e27b0e    4.7 GB    6 months ago\n",
		},
	}
	hw := hardware.Info{RAMGB: 32}
	models := catalog.Models() // catálogo embebido: contiene qwen2.5:7b

	got, err := EvaluateLocal(context.Background(), r, hw, models)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1 (uno conocido, uno filtrado); got %+v", len(got), got)
	}
	if got[0].Name != "qwen2.5:7b" {
		t.Errorf("Name = %q, want qwen2.5:7b", got[0].Name)
	}
	if got[0].LocalSize != "4.7 GB" {
		t.Errorf("LocalSize = %q, want 4.7 GB", got[0].LocalSize)
	}
	if got[0].Result.Model.Name != "qwen2.5:7b" {
		t.Errorf("Result.Model.Name = %q, want qwen2.5:7b", got[0].Result.Model.Name)
	}
}

func TestEvaluateLocal_SkipsCloud(t *testing.T) {
	r := &fakeRunner{
		output: map[string]string{
			"ollama list": "NAME              ID              SIZE      MODIFIED\n" +
				"gemma4:31b-cloud c382fbfbc73b    -         2 months ago\n" +
				"qwen3:8b         abc123          5.1 GB    yesterday\n",
		},
	}
	hw := hardware.Info{RAMGB: 32}
	models := []catalog.Model{
		{Name: "qwen3:8b", Family: "qwen3", Params: "8B", Quant: "Q4_K_M", SizeGB: 5.1},
	}

	got, _ := EvaluateLocal(context.Background(), r, hw, models)
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1 (cloud filtered); got %+v", len(got), got)
	}
	if got[0].Name != "qwen3:8b" {
		t.Errorf("Name = %q, want qwen3:8b", got[0].Name)
	}
}

func TestEvaluateLocal_RunnerError(t *testing.T) {
	r := &fakeRunner{
		err: map[string]error{
			"ollama list": fmt.Errorf("ollama not in PATH"),
		},
	}

	_, err := EvaluateLocal(context.Background(), r, hardware.Info{RAMGB: 32}, nil)
	if err == nil {
		t.Fatal("expected error when runner fails, got nil")
	}
}

// -- Format -----------------------------------------------------------

func TestFormat_BasicColumns(t *testing.T) {
	results := []Model{
		{Name: "qwen2.5:7b", LocalSize: "4.7 GB", Result: eval.Result{
			Model:   catalog.Model{Name: "qwen2.5:7b", Family: "qwen2.5", Params: "7B", Quant: "Q4_K_M", SizeGB: 4.7},
			Verdict: eval.Good,
			Backend: "CPU",
			NeedGB:  4.7 * 1.2,
			Reason:  "Cabe en RAM, fluido en CPU",
		}},
		{Name: "huge:70b", LocalSize: "43 GB", Result: eval.Result{
			Model:   catalog.Model{Name: "huge:70b", Family: "huge", Params: "70B", Quant: "Q4_K_M", SizeGB: 43},
			Verdict: eval.No,
			Backend: "—",
			NeedGB:  43 * 1.2,
			Reason:  "No cabe en memoria",
		}},
	}
	out := Format(results)

	for _, want := range []string{
		"qwen2.5:7b", "huge:70b", "4.7 GB", "43 GB",
		"Good", "No", "CPU", "—",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Format() missing %q\n---\n%s", want, out)
		}
	}
}

func TestFormat_Empty(t *testing.T) {
	out := Format(nil)
	if !strings.Contains(out, "no local") && !strings.Contains(out, "(none)") && out == "" {
		t.Errorf("Format(nil) = %q, want some message", out)
	}
}
