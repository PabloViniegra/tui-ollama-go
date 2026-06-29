package cmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/PabloViniegra/tui-ollama-go/internal/catalog"
	"github.com/PabloViniegra/tui-ollama-go/internal/eval"
	"github.com/PabloViniegra/tui-ollama-go/internal/hardware"
	"github.com/PabloViniegra/tui-ollama-go/internal/loader"
)

type fitOpts struct {
	Model       string
	AsJSON      bool
	AsExplain   bool
	PrintSchema bool
}

type fitOutput struct {
	Verdict     string        `json:"verdict"`
	Backend     string        `json:"backend"`
	NeedGB      float64       `json:"need_gb"`
	AvailableGB float64       `json:"available_gb"`
	Reason      string        `json:"reason"`
	Model       catalog.Model `json:"model"`
}

func runFit(args []string, src *loader.Source, schema []byte) int {
	opts, err := parseFitFlags(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 3
	}

	if opts.PrintSchema {
		fmt.Println(string(schema))
		return 0
	}

	hw := src.Detect(context.Background())
	models, fetchErr := src.Fetch(context.Background(), false, false, nil)
	if len(models) == 0 {
		if fetchErr != nil {
			fmt.Fprintf(os.Stderr, "error cargando catálogo: %v\n", fetchErr)
		} else {
			fmt.Fprintln(os.Stderr, "catálogo vacío")
		}
		return 3
	}
	mdl, ok := findModel(models, opts.Model)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown model %q — corré `ollama-fit --refresh` para actualizar el catálogo\n", opts.Model)
		return 3
	}
	out, code := fitReport(hw, mdl, opts.AsJSON, opts.AsExplain)
	fmt.Println(out)
	return code
}

func parseFitFlags(args []string) (fitOpts, error) {
	fs := flag.NewFlagSet("ollama-fit fit", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	asJSON := fs.Bool("json", false, "emitir el veredicto como JSON de una línea")
	asExplain := fs.Bool("explain", false, "mostrar el cálculo paso a paso")
	printSchema := fs.Bool("print-schema", false, "imprime el JSON Schema del output --json y sale")
	if err := fs.Parse(args); err != nil {
		return fitOpts{}, err
	}
	if !*printSchema && fs.NArg() != 1 {
		return fitOpts{}, fmt.Errorf("uso: ollama-fit fit [--json|--explain] <modelo>")
	}
	active := 0
	if *asJSON {
		active++
	}
	if *asExplain {
		active++
	}
	if *printSchema {
		active++
	}
	if active > 1 {
		return fitOpts{}, fmt.Errorf("--json, --explain y --print-schema son mutuamente excluyentes")
	}
	return fitOpts{
		Model:       fs.Arg(0),
		AsJSON:      *asJSON,
		AsExplain:   *asExplain,
		PrintSchema: *printSchema,
	}, nil
}

func findModel(models []catalog.Model, name string) (catalog.Model, bool) {
	for _, m := range models {
		if strings.EqualFold(m.Name, name) {
			return m, true
		}
	}
	return catalog.Model{}, false
}

func availableGB(hw hardware.Info) float64 {
	switch {
	case hw.GPU.VRAMGB > 0:
		return hw.GPU.VRAMGB
	case hw.AppleUnified:
		return eval.AppleGPUFraction * hw.RAMGB
	default:
		return hw.RAMGB
	}
}

func verdictExitCode(v eval.Verdict) int {
	switch v {
	case eval.Good:
		return 0
	case eval.Tight:
		return 1
	case eval.No:
		return 2
	default:
		return 3
	}
}

func fitReport(hw hardware.Info, mdl catalog.Model, asJSON, asExplain bool) (string, int) {
	r := eval.Evaluate(hw, mdl)
	available := availableGB(hw)
	code := verdictExitCode(r.Verdict)

	switch {
	case asExplain:
		return explainReport(hw, mdl, r, available), code
	case asJSON:
		payload := fitOutput{
			Verdict:     strings.ToLower(r.Verdict.String()),
			Backend:     r.Backend,
			NeedGB:      r.NeedGB,
			AvailableGB: available,
			Reason:      r.Reason,
			Model:       mdl,
		}
		b, err := json.Marshal(payload)
		if err != nil {
			return "", 3
		}
		return string(b), code
	}
	text := fmt.Sprintf("%s → %s\n  backend : %s\n  reason  : %s\n  need    : %.1f GB / available %.1f GB\n",
		r.Model.Name, r.Verdict, r.Backend, r.Reason, r.NeedGB, available)
	return text, code
}

func explainReport(hw hardware.Info, mdl catalog.Model, r eval.Result, available float64) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Model:     %s (%s, %s, %s)\n", mdl.Name, mdl.Family, mdl.Params, mdl.Quant)
	fmt.Fprintf(&sb, "Size:      %.1f GB\n", mdl.SizeGB)
	fmt.Fprintf(&sb, "Need:      %.2f GB  (size × 1.2 overhead)\n", r.NeedGB)
	fmt.Fprintf(&sb, "Available: %.2f GB  (%s)\n", available, hardwareSummary(hw))
	fmt.Fprintf(&sb, "Backend:   %s\n", r.Backend)
	fmt.Fprintf(&sb, "Rule:      %s\n", r.Reason)
	fmt.Fprintf(&sb, "Verdict:   %s\n", r.Verdict)
	return sb.String()
}

func hardwareSummary(hw hardware.Info) string {
	switch {
	case hw.GPU.VRAMGB > 0:
		return fmt.Sprintf("GPU %s, %.1f GB VRAM", strings.ToUpper(hw.GPU.Kind.String()), hw.GPU.VRAMGB)
	case hw.AppleUnified:
		return fmt.Sprintf("Apple Silicon unified, %.1f GB usable (%.0f%% de %.1f GB RAM)",
			eval.AppleGPUFraction*hw.RAMGB, eval.AppleGPUFraction*100, hw.RAMGB)
	default:
		return fmt.Sprintf("CPU, %.1f GB RAM", hw.RAMGB)
	}
}
