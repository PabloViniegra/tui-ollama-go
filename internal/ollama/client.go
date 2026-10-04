package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

type Metadata struct {
	Name          string
	SizeBytes     int64
	ContextLength int
	BlockCount    int
	KVHeads       int
	KeyLength     int
	ValueLength   int
}

type LocalModel struct {
	Name      string
	SizeBytes int64
}

type Metrics struct {
	LoadDuration       int64 `json:"load_duration"`
	PromptEvalCount    int64 `json:"prompt_eval_count"`
	PromptEvalDuration int64 `json:"prompt_eval_duration"`
	EvalCount          int64 `json:"eval_count"`
	EvalDuration       int64 `json:"eval_duration"`
}

func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = os.Getenv("OLLAMA_HOST")
	}
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	} else if !strings.Contains(baseURL, "://") {
		baseURL = "http://" + baseURL
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 10 * time.Minute},
	}
}

func (c *Client) ModelMetadata(ctx context.Context, name string) (Metadata, error) {
	model, err := c.LocalModel(ctx, name)
	if err != nil {
		return Metadata{}, err
	}
	var details struct {
		ModelInfo map[string]json.RawMessage `json:"model_info"`
	}
	if err := c.request(ctx, http.MethodPost, "/api/show", map[string]string{"model": model.Name}, &details); err != nil {
		return Metadata{}, err
	}
	return parseMetadata(model.Name, model.SizeBytes, details.ModelInfo)
}

func (c *Client) LocalModel(ctx context.Context, name string) (LocalModel, error) {
	var tags struct {
		Models []struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
		} `json:"models"`
	}
	if err := c.request(ctx, http.MethodGet, "/api/tags", nil, &tags); err != nil {
		return LocalModel{}, err
	}
	for _, model := range tags.Models {
		if strings.EqualFold(model.Name, name) || (!strings.Contains(name, ":") && strings.EqualFold(model.Name, name+":latest")) {
			if model.Size <= 0 {
				return LocalModel{}, fmt.Errorf("Ollama no informó el tamaño local de %q", model.Name)
			}
			return LocalModel{Name: model.Name, SizeBytes: model.Size}, nil
		}
	}
	return LocalModel{}, fmt.Errorf("el modelo %q no está instalado en Ollama; instálalo localmente, por ejemplo con `ollama pull %s` si está disponible", name, name)
}

func parseMetadata(name string, sizeBytes int64, info map[string]json.RawMessage) (Metadata, error) {
	var architecture string
	if err := json.Unmarshal(info["general.architecture"], &architecture); err != nil || architecture == "" {
		return Metadata{}, fmt.Errorf("Ollama no informó la arquitectura de %q", name)
	}
	prefix := architecture + "."
	readInt := func(field string) (int, error) {
		raw, ok := info[prefix+field]
		if !ok {
			return 0, fmt.Errorf("falta el metadato %s%s para %q", prefix, field, name)
		}
		var value int
		if err := json.Unmarshal(raw, &value); err != nil || value <= 0 {
			return 0, fmt.Errorf("metadato inválido %s%s para %q", prefix, field, name)
		}
		return value, nil
	}

	contextLength, err := readInt("context_length")
	if err != nil {
		return Metadata{}, err
	}
	blockCount, err := readInt("block_count")
	if err != nil {
		return Metadata{}, err
	}
	headCount, err := readInt("attention.head_count")
	if err != nil {
		return Metadata{}, err
	}
	var kvHeads int
	if _, ok := info[prefix+"attention.head_count_kv"]; ok {
		kvHeads, err = readInt("attention.head_count_kv")
		if err != nil {
			return Metadata{}, err
		}
	} else {
		kvHeads = headCount
	}
	var keyLength int
	if _, ok := info[prefix+"attention.key_length"]; ok {
		keyLength, err = readInt("attention.key_length")
		if err != nil {
			return Metadata{}, err
		}
	} else {
		embeddingLength, embeddingErr := readInt("embedding_length")
		if embeddingErr != nil || embeddingLength%headCount != 0 {
			return Metadata{}, fmt.Errorf("no se pudo determinar el tamaño de las cabezas de atención de %q", name)
		}
		keyLength = embeddingLength / headCount
	}
	var valueLength int
	if _, ok := info[prefix+"attention.value_length"]; ok {
		valueLength, err = readInt("attention.value_length")
		if err != nil {
			return Metadata{}, err
		}
	} else {
		valueLength = keyLength
	}
	return Metadata{
		Name:          name,
		SizeBytes:     sizeBytes,
		ContextLength: contextLength,
		BlockCount:    blockCount,
		KVHeads:       kvHeads,
		KeyLength:     keyLength,
		ValueLength:   valueLength,
	}, nil
}

func (c *Client) Generate(ctx context.Context, model, prompt string, contextLength, numPredict int) (Metrics, error) {
	request := struct {
		Model   string `json:"model"`
		Prompt  string `json:"prompt"`
		Stream  bool   `json:"stream"`
		Options struct {
			NumCtx      int     `json:"num_ctx"`
			NumPredict  int     `json:"num_predict"`
			Seed        int     `json:"seed"`
			Temperature float64 `json:"temperature"`
		} `json:"options"`
	}{Model: model, Prompt: prompt}
	request.Options.NumCtx = contextLength
	request.Options.NumPredict = numPredict
	request.Options.Seed = 42
	request.Options.Temperature = 0
	var metrics Metrics
	if err := c.request(ctx, http.MethodPost, "/api/generate", request, &metrics); err != nil {
		return Metrics{}, err
	}
	return metrics, nil
}

func (c *Client) request(ctx context.Context, method, path string, body, response any) error {
	var requestBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		requestBody = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, requestBody)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("ollama %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("ollama %s: HTTP %d: %s", path, resp.StatusCode, strings.TrimSpace(string(message)))
	}
	if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
		return fmt.Errorf("ollama %s: respuesta inválida: %w", path, err)
	}
	return nil
}
