package main

import (
	_ "embed"
	"os"

	"github.com/PabloViniegra/tui-ollama-go/internal/cmd"
)

//go:embed assets/fit_output.schema.json
var fitOutputSchema []byte

// Estas variables se sobreescriben en build con -ldflags "-X main.version=...".
// `.goreleaser.yaml` las inyecta en cada release.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	os.Exit(cmd.Run(os.Args, version, commit, date, fitOutputSchema))
}

func runMain(args []string) int {
	return cmd.Run(args, version, commit, date, fitOutputSchema)
}
