package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestModelMetadataReadsInstalledSizeAndArchitecture(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			if r.Method != http.MethodGet {
				t.Errorf("/api/tags method = %s, want GET", r.Method)
				return
			}
			_, _ = w.Write([]byte(`{"models":[{"name":"llama3.1:8b","size":5000000000}]}`))
		case "/api/show":
			var request struct {
				Model string `json:"model"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode show request: %v", err)
				return
			}
			if request.Model != "llama3.1:8b" {
				t.Errorf("show model = %q", request.Model)
			}
			_, _ = w.Write([]byte(`{"model_info":{"general.architecture":"llama","llama.context_length":131072,"llama.block_count":32,"llama.embedding_length":4096,"llama.attention.head_count":32,"llama.attention.head_count_kv":8,"llama.attention.key_length":128,"llama.attention.value_length":128}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	metadata, err := NewClient(server.URL).ModelMetadata(context.Background(), "llama3.1:8b")
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Name != "llama3.1:8b" || metadata.SizeBytes != 5_000_000_000 {
		t.Errorf("metadata identity/size = %+v", metadata)
	}
	if metadata.ContextLength != 131072 || metadata.BlockCount != 32 || metadata.KVHeads != 8 {
		t.Errorf("metadata architecture = %+v", metadata)
	}
	if metadata.KeyLength != 128 || metadata.ValueLength != 128 {
		t.Errorf("metadata cache dimensions = %+v", metadata)
	}
}

func TestModelMetadataRequiresInstalledModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer server.Close()

	if _, err := NewClient(server.URL).ModelMetadata(context.Background(), "missing:7b"); err == nil {
		t.Fatal("expected missing model error")
	}
}

func TestParseMetadataDefaultsToFullAttention(t *testing.T) {
	metadata, err := parseMetadata("m:tag", 5_000_000_000, map[string]json.RawMessage{
		"general.architecture":       json.RawMessage(`"llama"`),
		"llama.context_length":       json.RawMessage(`8192`),
		"llama.block_count":          json.RawMessage(`32`),
		"llama.embedding_length":     json.RawMessage(`4096`),
		"llama.attention.head_count": json.RawMessage(`32`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if metadata.KVHeads != 32 || metadata.KeyLength != 128 || metadata.ValueLength != 128 {
		t.Errorf("fallback dimensions = %+v", metadata)
	}
}

func TestParseMetadataRejectsInvalidExplicitKVHeads(t *testing.T) {
	_, err := parseMetadata("m:tag", 5_000_000_000, map[string]json.RawMessage{
		"general.architecture":          json.RawMessage(`"llama"`),
		"llama.context_length":          json.RawMessage(`8192`),
		"llama.block_count":             json.RawMessage(`32`),
		"llama.attention.head_count":    json.RawMessage(`32`),
		"llama.attention.head_count_kv": json.RawMessage(`0`),
		"llama.attention.key_length":    json.RawMessage(`128`),
		"llama.attention.value_length":  json.RawMessage(`128`),
	})
	if err == nil {
		t.Fatal("expected invalid KV head count error")
	}
}

func TestGenerateSendsBenchmarkOptionsAndReadsMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" || r.Method != http.MethodPost {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		var request struct {
			Model   string `json:"model"`
			Prompt  string `json:"prompt"`
			Stream  bool   `json:"stream"`
			Options struct {
				NumCtx      int     `json:"num_ctx"`
				NumPredict  int     `json:"num_predict"`
				Seed        int     `json:"seed"`
				Temperature float64 `json:"temperature"`
			} `json:"options"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode generate request: %v", err)
			return
		}
		if request.Model != "llama3.1:8b" || request.Prompt == "" || request.Stream {
			t.Errorf("generate request = %+v", request)
		}
		if request.Options.NumCtx != 8192 || request.Options.NumPredict != 128 || request.Options.Seed != 42 || request.Options.Temperature != 0 {
			t.Errorf("generate options = %+v", request.Options)
		}
		_, _ = w.Write([]byte(`{"load_duration":1000000000,"prompt_eval_count":20,"prompt_eval_duration":500000000,"eval_count":100,"eval_duration":2000000000}`))
	}))
	defer server.Close()

	metrics, err := NewClient(server.URL).Generate(context.Background(), "llama3.1:8b", "test prompt", 8192, 128)
	if err != nil {
		t.Fatal(err)
	}
	if metrics.LoadDuration != 1_000_000_000 || metrics.PromptEvalCount != 20 || metrics.EvalCount != 100 || metrics.EvalDuration != 2_000_000_000 {
		t.Errorf("metrics = %+v", metrics)
	}
}
