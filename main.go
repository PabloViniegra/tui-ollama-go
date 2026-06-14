package main

import (
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
	refresh := flag.Bool("refresh", false, "ignora la caché y vuelve a scrapear ollama.com")
	offline := flag.Bool("offline", false, "no usa red; usa el catálogo embebido de respaldo")
	flag.Parse()

	fmt.Fprintln(os.Stderr, "Detectando hardware…")
	hw := hardware.Detect()

	models, err := catalog.Fetch(*refresh, *offline, func(s string) {
		fmt.Fprintln(os.Stderr, s)
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "aviso:", err)
	}
	if len(models) == 0 {
		fmt.Fprintln(os.Stderr, "no se pudo obtener ningún modelo")
		os.Exit(1)
	}

	results := make([]eval.Result, 0, len(models))
	for _, mdl := range models {
		results = append(results, eval.Evaluate(hw, mdl))
	}

	p := tea.NewProgram(tui.New(hw, results), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
