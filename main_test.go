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
