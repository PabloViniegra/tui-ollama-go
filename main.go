package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/PabloViniegra/tui-ollama-go/internal/catalog"
	"github.com/PabloViniegra/tui-ollama-go/internal/eval"
	"github.com/PabloViniegra/tui-ollama-go/internal/hardware"
	"github.com/PabloViniegra/tui-ollama-go/internal/tui"
)

func main() {
	os.Exit(runMain(os.Args))
}

func runMain(args []string) int {
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
	fs := flag.NewFlagSet("ollama-fit fit", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return 3
	}
	if fs.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "uso: ollama-fit fit <modelo>\n")
		return 3
	}
	name := fs.Arg(0)

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
	mdl, ok := findModel(models, name)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown model %q — corré `ollama-fit --refresh` para actualizar el catálogo\n", name)
		return 3
	}
	r := eval.Evaluate(hw, mdl)
	available := availableGB(hw)
	fmt.Printf("%s → %s\n", r.Model.Name, r.Verdict)
	fmt.Printf("  backend : %s\n", r.Backend)
	fmt.Printf("  reason  : %s\n", r.Reason)
	fmt.Printf("  need    : %.1f GB / available %.1f GB\n", r.NeedGB, available)
	return verdictExitCode(r.Verdict)
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
