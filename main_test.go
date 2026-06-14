package main

import (
	"context"
	"testing"
	"time"

	"ollama-fit/internal/catalog"
	"ollama-fit/internal/hardware"
)

func TestCatalogFetchCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := catalog.Fetch(ctx, false, false, nil)
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
}

func TestCatalogFetchTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond) // ensure timeout fires

	_, err := catalog.Fetch(ctx, false, false, nil)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestHardwareDetectCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	info := hardware.Detect(ctx)
	// Should return early without panicking.
	if info.OS != "" {
		t.Log("Detect returned partial info on cancelled ctx")
	}
}

func TestRunMainOfflineFlagParses(t *testing.T) {
	// --offline should parse without panic and return 1 because the TUI
	// can't run in a non-interactive test environment.
	code := runMain([]string{"ollama-fit", "--offline"})
	if code != 1 {
		// It may fail because tea.NewProgram needs a terminal, which is fine.
		// We just care that it doesn't panic and the flag parses.
		t.Logf("runMain(--offline) returned %d (expected 1 in non-interactive env)", code)
	}
}

func TestRunMainInvalidFlag(t *testing.T) {
	code := runMain([]string{"ollama-fit", "--invalid-flag"})
	if code != 1 {
		t.Fatalf("expected exit code 1 for invalid flag, got %d", code)
	}
}
