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
