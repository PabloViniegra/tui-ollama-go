package cmd

import (
	"fmt"
	"runtime"
)

func printVersion(version, commit, date string) string {
	return fmt.Sprintf("ollama-fit %s (%s/%s, %s, commit=%s, built=%s)",
		version, runtime.GOOS, runtime.GOARCH, runtime.Version(), commit, date)
}
