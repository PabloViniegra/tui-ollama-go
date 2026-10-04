package cmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/PabloViniegra/tui-ollama-go/internal/ollama"
)

const benchmarkPrompt = "Cuenta en orden desde 1, separando cada número con un espacio. Continúa hasta alcanzar el límite de generación."

type benchOpts struct {
	Model      string
	Runs       int
	Context    int
	NumPredict int
}

func parseBenchFlags(args []string) (benchOpts, error) {
	fs := flag.NewFlagSet("ollama-fit bench", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	runs := fs.Int("runs", 3, "número de ejecuciones")
	contextTokens := fs.Int("context", 4096, "ventana de contexto en tokens")
	numPredict := fs.Int("num-predict", 128, "máximo de tokens generados por ejecución")
	if err := fs.Parse(args); err != nil {
		return benchOpts{}, err
	}
	if fs.NArg() != 1 {
		return benchOpts{}, fmt.Errorf("uso: ollama-fit bench [--runs n] [--context tokens] [--num-predict tokens] <modelo>")
	}
	if *runs < 1 || *contextTokens < 1 || *numPredict < 1 {
		return benchOpts{}, fmt.Errorf("--runs, --context y --num-predict deben ser positivos")
	}
	return benchOpts{Model: fs.Arg(0), Runs: *runs, Context: *contextTokens, NumPredict: *numPredict}, nil
}

func runBench(args []string) int {
	opts, err := parseBenchFlags(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 3
	}
	client := ollama.NewClient("")
	model, err := client.LocalModel(context.Background(), opts.Model)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 3
	}
	metrics := make([]ollama.Metrics, 0, opts.Runs)
	for run := 0; run < opts.Runs; run++ {
		result, err := client.Generate(context.Background(), model.Name, benchmarkPrompt, opts.Context, opts.NumPredict)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error en la ejecución %d: %v\n", run+1, err)
			return 3
		}
		metrics = append(metrics, result)
	}
	fmt.Print(benchmarkReport(model.Name, opts, metrics))
	return 0
}

func benchmarkReport(model string, opts benchOpts, metrics []ollama.Metrics) string {
	var promptTokens, promptDuration, outputTokens, outputDuration int64
	for _, result := range metrics {
		promptTokens += result.PromptEvalCount
		promptDuration += result.PromptEvalDuration
		outputTokens += result.EvalCount
		outputDuration += result.EvalDuration
	}
	var promptRate, outputRate float64
	if promptDuration > 0 {
		promptRate = float64(promptTokens) / (float64(promptDuration) / 1e9)
	}
	if outputDuration > 0 {
		outputRate = float64(outputTokens) / (float64(outputDuration) / 1e9)
	}
	var firstLoad float64
	if len(metrics) > 0 {
		firstLoad = float64(metrics[0].LoadDuration) / 1e9
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Modelo: %s\nContexto: %d tokens\nEjecuciones: %d\n", model, opts.Context, len(metrics))
	fmt.Fprintf(&sb, "Límite por ejecución: %d tokens\n", opts.NumPredict)
	fmt.Fprintf(&sb, "Carga (primera ejecución): %.2f s\n", firstLoad)
	fmt.Fprintf(&sb, "Prompt: %d tokens, %.1f tokens/s\n", promptTokens, promptRate)
	fmt.Fprintf(&sb, "Generación: %d tokens, %.1f tokens/s\n", outputTokens, outputRate)
	return sb.String()
}
