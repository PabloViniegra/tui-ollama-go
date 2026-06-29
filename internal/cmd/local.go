package cmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/PabloViniegra/tui-ollama-go/internal/catalog"
	"github.com/PabloViniegra/tui-ollama-go/internal/hardware"
	"github.com/PabloViniegra/tui-ollama-go/internal/loader"
	"github.com/PabloViniegra/tui-ollama-go/internal/locallist"
)

type execLocalRunner struct{}

func (execLocalRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return "", fmt.Errorf("exec %s: %w", name, err)
	}
	return string(out), nil
}

func runLocal(args []string, src *loader.Source, runner locallist.CommandRunner) int {
	fs := flag.NewFlagSet("ollama-fit local", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "uso: ollama-fit local")
		return 3
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "uso: ollama-fit local")
		return 3
	}

	hw := src.Detect(context.Background())
	models, err := src.Fetch(context.Background(), false, false, nil)
	if err != nil || len(models) == 0 {
		models = catalog.Models()
	}
	out, err := localReport(context.Background(), runner, hw, models)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 3
	}
	fmt.Print(out)
	return 0
}

func localReport(ctx context.Context, runner locallist.CommandRunner, hw hardware.Info, models []catalog.Model) (string, error) {
	results, err := locallist.EvaluateLocal(ctx, runner, hw, models)
	if err != nil {
		return "", err
	}
	return locallist.Format(results), nil
}
