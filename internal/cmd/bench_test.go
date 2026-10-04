package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseBenchFlagsDefaults(t *testing.T) {
	opts, err := parseBenchFlags([]string{"qwen2.5:7b"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.Runs != 3 || opts.Context != 4096 || opts.NumPredict != 128 {
		t.Errorf("defaults = %+v", opts)
	}
}

func TestParseBenchFlagsRejectsInvalidValues(t *testing.T) {
	for _, args := range [][]string{
		{"--runs", "0", "model"},
		{"--context", "0", "model"},
		{"--num-predict", "-1", "model"},
		{},
		{"model", "extra"},
	} {
		t.Run(fmt.Sprint(args), func(t *testing.T) {
			if _, err := parseBenchFlags(args); err == nil {
				t.Fatal("expected invalid arguments error")
			}
		})
	}
}

func TestRunBenchUsesLocalAPIAndAggregatesMetrics(t *testing.T) {
	var generationRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_, _ = w.Write([]byte(`{"models":[{"name":"model:latest","size":5000000000}]}`))
		case "/api/generate":
			generationRequests++
			var request struct {
				Model   string `json:"model"`
				Stream  bool   `json:"stream"`
				Options struct {
					NumCtx     int `json:"num_ctx"`
					NumPredict int `json:"num_predict"`
				} `json:"options"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			if request.Model != "model:latest" || request.Stream || request.Options.NumCtx != 2048 || request.Options.NumPredict != 16 {
				t.Errorf("generate request = %+v", request)
			}
			var load int64
			if generationRequests == 1 {
				load = 1_000_000_000
			}
			_, _ = fmt.Fprintf(w, `{"load_duration":%d,"prompt_eval_count":10,"prompt_eval_duration":1000000000,"eval_count":20,"eval_duration":2000000000}`, load)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("OLLAMA_HOST", server.URL)

	output := captureStdout(t, func() {
		if code := Run([]string{"ollama-fit", "bench", "--runs", "2", "--context", "2048", "--num-predict", "16", "model"}, "dev", "none", "unknown", nil); code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
	})
	if generationRequests != 2 {
		t.Fatalf("generate called %d times, want 2", generationRequests)
	}
	for _, want := range []string{
		"Modelo: model:latest", "Contexto: 2048 tokens", "Ejecuciones: 2", "Límite por ejecución: 16 tokens",
		"Carga (primera ejecución): 1.00 s", "Prompt: 20 tokens, 10.0 tokens/s",
		"Generación: 40 tokens, 10.0 tokens/s",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q\n%s", want, output)
		}
	}
}

func TestRunBenchDoesNotGenerateForMissingModel(t *testing.T) {
	var generationRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_, _ = w.Write([]byte(`{"models":[]}`))
			return
		}
		generationRequests++
		http.Error(w, "unexpected generation", http.StatusInternalServerError)
	}))
	defer server.Close()
	t.Setenv("OLLAMA_HOST", server.URL)

	if code := runBench([]string{"missing:7b"}); code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
	if generationRequests != 0 {
		t.Errorf("generated %d times for missing model", generationRequests)
	}
}
