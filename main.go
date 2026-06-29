package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/PabloViniegra/tui-ollama-go/internal/catalog"
	"github.com/PabloViniegra/tui-ollama-go/internal/eval"
	"github.com/PabloViniegra/tui-ollama-go/internal/hardware"
	"github.com/PabloViniegra/tui-ollama-go/internal/tui"
)

// Estas variables se sobreescriben en build con -ldflags "-X main.version=...".
// `.goreleaser.yaml` las inyecta en cada release.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	os.Exit(runMain(os.Args))
}

func runMain(args []string) int {
	if len(args) > 1 && (args[1] == "--version" || args[1] == "-V") {
		fmt.Println(printVersion())
		return 0
	}
	if len(args) > 1 && args[1] == "fit" {
		return runFit(args[2:])
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	refresh := fs.Bool("refresh", false, "ignora la caché y vuelve a scrapear ollama.com")
	offline := fs.Bool("offline", false, "no usa red; usa el catálogo embebido de respaldo")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	loader := func() (hardware.Info, []eval.Result, error) {
		hw := hardware.Detect(context.Background())
		models, err := catalog.Fetch(context.Background(), *refresh, *offline, nil)
		if err != nil {
			return hw, nil, err
		}
		results := make([]eval.Result, 0, len(models))
		for _, mdl := range models {
			results = append(results, eval.Evaluate(hw, mdl))
		}
		return hw, results, nil
	}

	p := tea.NewProgram(tui.NewAsync(loader), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

// runFit implementa el subcomando `fit <modelo>`: detecta hardware, busca el
// modelo en el catálogo, evalúa con la misma heurística que el TUI e imprime el
// veredicto. No abre la TUI.
func runFit(args []string) int {
	opts, err := parseFitFlags(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 3
	}

	hw := hardware.Detect(context.Background())
	models, fetchErr := catalog.Fetch(context.Background(), false, false, nil)
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

// fitOpts es el resultado de parsear los flags del subcomando `fit`.
type fitOpts struct {
	Model     string
	AsJSON    bool
	AsExplain bool
}

// parseFitFlags valida y extrae los flags de `fit`. Errores de uso y de
// combinación inválida devuelven error (no tocan stderr; el caller decide).
func parseFitFlags(args []string) (fitOpts, error) {
	fs := flag.NewFlagSet("ollama-fit fit", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // errores se propagan por la tupla de retorno
	asJSON := fs.Bool("json", false, "emitir el veredicto como JSON de una línea")
	asExplain := fs.Bool("explain", false, "mostrar el cálculo paso a paso")
	if err := fs.Parse(args); err != nil {
		return fitOpts{}, err
	}
	if fs.NArg() != 1 {
		return fitOpts{}, fmt.Errorf("uso: ollama-fit fit [--json|--explain] <modelo>")
	}
	if *asJSON && *asExplain {
		return fitOpts{}, fmt.Errorf("--json y --explain son mutuamente excluyentes")
	}
	return fitOpts{Model: fs.Arg(0), AsJSON: *asJSON, AsExplain: *asExplain}, nil
}

// findModel busca un modelo por nombre, case-insensitive y exacto.
func findModel(models []catalog.Model, name string) (catalog.Model, bool) {
	for _, m := range models {
		if strings.EqualFold(m.Name, name) {
			return m, true
		}
	}
	return catalog.Model{}, false
}

// availableGB devuelve la memoria "acelerada" disponible (mismo cálculo que el
// TUI muestra en el panel de hardware). Solo se usa para el output del CLI.
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

// verdictExitCode mapea el veredicto a un código de salida shell-friendly:
// 0=Good, 1=Tight, 2=No, 3=Error.
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

// fitOutput es la forma estable del veredicto para consumidores externos
// (scripts CI, jq, etc.). Los verbos del campo Verdict van en minúsculas para
// ser estables ante cambios de idioma del TUI.
type fitOutput struct {
	Verdict     string        `json:"verdict"`
	Backend     string        `json:"backend"`
	NeedGB      float64       `json:"need_gb"`
	AvailableGB float64       `json:"available_gb"`
	Reason      string        `json:"reason"`
	Model       catalog.Model `json:"model"`
}

// fitReport evalúa el modelo y devuelve la salida formateada (texto, JSON o
// explicación detallada) junto al exit code correspondiente al veredicto.
// Si ambos asJSON y asExplain son true, tiene precedencia asExplain (no son
// combinables — se valida antes en parseFitFlags).
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

// explainReport describe el cálculo paso a paso para que un humano entienda
// por qué el veredicto es el que es.
func explainReport(hw hardware.Info, mdl catalog.Model, r eval.Result, available float64) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Model:     %s (%s, %s, %s)\n", mdl.Name, mdl.Family, mdl.Params, mdl.Quant)
	fmt.Fprintf(&sb, "Size:      %.1f GB\n", mdl.SizeGB)
	fmt.Fprintf(&sb, "Need:      %.2f GB  (size × 1.2 overhead)\n", r.NeedGB) // 1.2 = eval.overhead
	fmt.Fprintf(&sb, "Available: %.2f GB  (%s)\n", available, hardwareSummary(hw))
	fmt.Fprintf(&sb, "Backend:   %s\n", r.Backend)
	fmt.Fprintf(&sb, "Rule:      %s\n", r.Reason)
	fmt.Fprintf(&sb, "Verdict:   %s\n", r.Verdict)
	return sb.String()
}

// hardwareSummary describe de dónde sale la memoria "acelerada".
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

// printVersion devuelve la cadena que se imprime con --version / -V.
// El valor por defecto "dev" se reemplaza en build con -ldflags
// "-X main.version=<v> -X main.commit=<sha> -X main.date=<ts>".
func printVersion() string {
	return fmt.Sprintf("ollama-fit %s (%s/%s, %s, commit=%s, built=%s)",
		version, runtime.GOOS, runtime.GOARCH, runtime.Version(), commit, date)
}
