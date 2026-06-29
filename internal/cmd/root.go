package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/PabloViniegra/tui-ollama-go/internal/eval"
	"github.com/PabloViniegra/tui-ollama-go/internal/hardware"
	"github.com/PabloViniegra/tui-ollama-go/internal/loader"
	"github.com/PabloViniegra/tui-ollama-go/internal/tui"
)

func Run(args []string, version, commit, date string, schema []byte) int {
	if len(args) > 1 && (args[1] == "--version" || args[1] == "-V") {
		fmt.Println(printVersion(version, commit, date))
		return 0
	}
	src := loader.Default()
	if len(args) > 1 && args[1] == "fit" {
		return runFit(args[2:], src, schema)
	}
	if len(args) > 1 && args[1] == "doctor" {
		return runDoctor(execDoctorRunner{})
	}
	if len(args) > 1 && args[1] == "local" {
		return runLocal(args[2:], src, execLocalRunner{})
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	refresh := fs.Bool("refresh", false, "ignora la caché y vuelve a scrapear ollama.com")
	offline := fs.Bool("offline", false, "no usa red; usa el catálogo embebido de respaldo")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	tuiLoader := func() (hardware.Info, []eval.Result, error) {
		hw := src.Detect(context.Background())
		models, err := src.Fetch(context.Background(), *refresh, *offline, nil)
		if err != nil {
			return hw, nil, err
		}
		results := make([]eval.Result, 0, len(models))
		for _, mdl := range models {
			results = append(results, eval.Evaluate(hw, mdl))
		}
		return hw, results, nil
	}

	p := tea.NewProgram(tui.NewAsync(tuiLoader), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}
