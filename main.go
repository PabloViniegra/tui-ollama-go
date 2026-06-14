package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"ollama-fit/internal/catalog"
	"ollama-fit/internal/eval"
	"ollama-fit/internal/hardware"
	"ollama-fit/internal/tui"
)

func main() {
	os.Exit(runMain(os.Args))
}

func runMain(args []string) int {
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	refresh := fs.Bool("refresh", false, "ignora la caché y vuelve a scrapear ollama.com")
	offline := fs.Bool("offline", false, "no usa red; usa el catálogo embebido de respaldo")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	fmt.Fprintln(os.Stderr, tui.MsgDetectingHardware)
	hw := hardware.Detect(context.Background())

	models, err := catalog.Fetch(context.Background(), *refresh, *offline, func(s string) {
		fmt.Fprintln(os.Stderr, s)
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "aviso:", err)
	}
	if len(models) == 0 {
		fmt.Fprintln(os.Stderr, "no se pudo obtener ningún modelo")
		return 1
	}

	results := make([]eval.Result, 0, len(models))
	for _, mdl := range models {
		results = append(results, eval.Evaluate(hw, mdl))
	}

	p := tea.NewProgram(tui.New(hw, results), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}
