package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseSizeGB(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"4.9GB · 128K · Text", 4.9},
		{"16 GB", 16.0},
		{"1.5 GB", 1.5},
		{"512 MB", 0.5},
		{"1024 MB", 1.0},
		{"no size here", 0},
		{"", 0},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := parseSizeGB(tc.input)
			if got != tc.want {
				t.Errorf("parseSizeGB(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseParams(t *testing.T) {
	tests := []struct {
		tag  string
		want string
	}{
		{"llama3.1:8b", "8B"},
		{"qwen2.5:0.5b", "0.5B"},
		{"gemma3:270m", "270M"},
		{"mistral:7b", "7B"},
		{"codellama:34b", "34B"},
		{"mixtral:8x7b", "7B"}, // regex finds "7b" in "8x7b" (skips "8x")
		{"nomic-embed-text", "—"},
		{"unknown", "—"},
	}
	for _, tc := range tests {
		t.Run(tc.tag, func(t *testing.T) {
			got := parseParams(tc.tag)
			if got != tc.want {
				t.Errorf("parseParams(%q) = %q, want %q", tc.tag, got, tc.want)
			}
		})
	}
}

func TestParseQuant(t *testing.T) {
	tests := []struct {
		tag  string
		want string
	}{
		{"llama3.1:8b-q4_K_M", "Q4_K_M"},
		{"mistral:7b-fp16", "FP16"},
		{"phi3:3.8b-bf16", "BF16"},
		{"gemma3:4b-q8_0", "Q8_0"},
		{"llama3.1:8b", "default"},
		{"tinyllama:1.1b", "default"},
		{"model:tag-f16", "F16"},
	}
	for _, tc := range tests {
		t.Run(tc.tag, func(t *testing.T) {
			got := parseQuant(tc.tag)
			if got != tc.want {
				t.Errorf("parseQuant(%q) = %q, want %q", tc.tag, got, tc.want)
			}
		})
	}
}

func TestHumanAge(t *testing.T) {
	tests := []struct {
		offset time.Duration
		want   string
	}{
		{-30 * time.Second, "hace segundos"},
		{-30 * time.Minute, "hace 30 min"},
		{-2 * time.Hour, "hace 2 h"},
		{-25 * time.Hour, "hace 25 h"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := humanAge(time.Now().Add(tc.offset))
			if got != tc.want {
				t.Errorf("humanAge(now%v) = %q, want %q", tc.offset, got, tc.want)
			}
		})
	}
}

func TestFetchOffline(t *testing.T) {
	models, err := Fetch(false, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := Models()
	if len(models) != len(want) {
		t.Errorf("got %d models, want %d", len(models), len(want))
	}
}

func TestFetchOfflineWithProgress(t *testing.T) {
	var msgs []string
	_, err := Fetch(false, true, func(s string) { msgs = append(msgs, s) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) == 0 {
		t.Error("expected at least one progress message")
	}
}

func TestSaveAndLoadCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.json")

	models := []Model{
		{Name: "test:7b", Family: "test", Params: "7B", Quant: "Q4_K_M", SizeGB: 4.1},
		{Name: "test:13b", Family: "test", Params: "13B", Quant: "Q4_0", SizeGB: 7.4},
	}

	if err := saveCache(path, models); err != nil {
		t.Fatalf("saveCache: %v", err)
	}

	cf, ok := loadCache(path)
	if !ok {
		t.Fatal("loadCache returned false for valid cache")
	}
	if len(cf.Models) != len(models) {
		t.Fatalf("got %d models, want %d", len(cf.Models), len(models))
	}
	if cf.Models[0].Name != "test:7b" {
		t.Errorf("got %q, want %q", cf.Models[0].Name, "test:7b")
	}
	if time.Since(cf.FetchedAt) > time.Second {
		t.Error("FetchedAt is too old")
	}
}

func TestLoadCacheMissing(t *testing.T) {
	_, ok := loadCache("/nonexistent/path/that/does/not/exist/catalog.json")
	if ok {
		t.Error("expected false for missing file")
	}
}

func TestLoadCacheInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{invalid json}"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, ok := loadCache(path)
	if ok {
		t.Error("expected false for invalid JSON")
	}
}

func TestCachePath(t *testing.T) {
	p := cachePath()
	if p == "" {
		t.Error("cachePath() returned empty string")
	}
	if !strings.Contains(p, "ollama-fit") {
		t.Errorf("cachePath() = %q, expected to contain 'ollama-fit'", p)
	}
}
