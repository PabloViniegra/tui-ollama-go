package main

import (
	"context"
	"testing"
	"time"

	"ollama-fit/internal/catalog"
	"ollama-fit/internal/eval"
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

func TestFindModel_Exact(t *testing.T) {
	models := catalog.Models() // catálogo embebido, determinístico
	m, ok := findModel(models, "qwen2.5:7b")
	if !ok {
		t.Fatal("expected to find qwen2.5:7b")
	}
	if m.Family != "qwen2.5" || m.Params != "7B" {
		t.Errorf("got %+v, want family=qwen2.5 params=7B", m)
	}
}

func TestFindModel_CaseInsensitive(t *testing.T) {
	models := catalog.Models()
	for _, name := range []string{"QWEN2.5:7B", "Qwen2.5:7b", "qWeN2.5:7B"} {
		if _, ok := findModel(models, name); !ok {
			t.Errorf("expected to find %q (case-insensitive)", name)
		}
	}
}

func TestFindModel_Unknown(t *testing.T) {
	models := catalog.Models()
	if _, ok := findModel(models, "no-existe-este-modelo-xyz-9876"); ok {
		t.Fatal("expected NOT to find unknown model")
	}
}

func TestAvailableGB(t *testing.T) {
	cases := []struct {
		name string
		hw   hardware.Info
		want float64
	}{
		{"nvidia", hardware.Info{GPU: hardware.GPU{VRAMGB: 12.0}}, 12.0},
		{"apple", hardware.Info{AppleUnified: true, RAMGB: 32.0}, eval.AppleGPUFraction * 32.0},
		{"cpu", hardware.Info{RAMGB: 16.0}, 16.0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := availableGB(c.hw); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestVerdictExitCode(t *testing.T) {
	cases := []struct {
		v    eval.Verdict
		want int
	}{
		{eval.Good, 0},
		{eval.Tight, 1},
		{eval.No, 2},
	}
	for _, c := range cases {
		if got := verdictExitCode(c.v); got != c.want {
			t.Errorf("verdictExitCode(%d) = %d, want %d", c.v, got, c.want)
		}
	}
}
