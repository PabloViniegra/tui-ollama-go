package catalog

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

//go:embed testdata/library.html.golden testdata/model.html.golden
var testdataFS embed.FS

// ----- existing tests (preserved) -----

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
	models, err := Fetch(context.Background(), false, true, nil)
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
	_, err := Fetch(context.Background(), false, true, func(s string) { msgs = append(msgs, s) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) == 0 {
		t.Error("expected at least one progress message")
	}
}

func TestFetchCacheMissFallsBackToEmbedded(t *testing.T) {
	// With refresh=true the cache is ignored; if network is unavailable
	// and there is no cache, it falls back to embedded models.
	// We test the offline=true path separately, so this is a smoke test.
	models, err := Fetch(context.Background(), true, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) == 0 {
		t.Error("expected non-empty embedded catalog")
	}
}

func TestSaveCacheMkdirError(t *testing.T) {
	// Try to save to a path that is not a directory to trigger MkdirAll error.
	path := "/dev/null/catalog.json"
	models := []Model{{Name: "test", Family: "test", Params: "7B", Quant: "Q4_K_M", SizeGB: 1.0}}
	err := saveCache(path, models)
	if err == nil {
		t.Fatal("expected error for invalid path, got nil")
	}
}

func TestHumanAgeNow(t *testing.T) {
	got := humanAge(time.Now())
	if got != "hace segundos" {
		t.Fatalf("humanAge(now) = %q, want hace segundos", got)
	}
}

func TestCachePathFallback(t *testing.T) {
	// Force os.UserCacheDir to fail by clearing HOME.
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", "")
	defer os.Setenv("HOME", oldHome)

	p := cachePath()
	if p == "" {
		t.Fatal("cachePath returned empty string when HOME is empty")
	}
	if !strings.Contains(p, "tmp") && !strings.Contains(p, "Temp") {
		t.Fatalf("cachePath() = %q, expected to fall back to temp dir", p)
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

// ----- new TDD tests (RED) -----

// fakeDoer implements HTTPDoer for tests.
type fakeDoer struct {
	resp map[string]*http.Response
	err  map[string]error
}

func (f *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	url := req.URL.String()
	if e, ok := f.err[url]; ok {
		return nil, e
	}
	if r, ok := f.resp[url]; ok {
		return r, nil
	}
	return nil, errors.New("no response for " + url)
}

func TestFakeDoerImplementsHTTPDoer(t *testing.T) {
	var _ HTTPDoer = &fakeDoer{}
}

func TestFakeDoerHappyPath(t *testing.T) {
	f := &fakeDoer{
		resp: map[string]*http.Response{
			"https://example.com": {
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString("hello")),
			},
		},
	}
	req, _ := http.NewRequest("GET", "https://example.com", nil)
	resp, err := f.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestFakeDoerErrorInjection(t *testing.T) {
	f := &fakeDoer{
		err: map[string]error{
			"https://example.com": errors.New("network error"),
		},
	}
	req, _ := http.NewRequest("GET", "https://example.com", nil)
	_, err := f.Do(req)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestScrapeLibraryWithFakeDoer(t *testing.T) {
	f := &fakeDoer{
		resp: map[string]*http.Response{
			libraryURL: {
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`<html><body><a href="/library/llama3">llama3</a></body></html>`)),
			},
		},
	}
	names, err := scrapeLibrary(context.Background(), f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 1 || names[0] != "llama3" {
		t.Fatalf("names = %v, want [llama3]", names)
	}
}

func TestScrapeLibraryWithFixture(t *testing.T) {
	b, err := testdataFS.ReadFile("testdata/library.html.golden")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	f := &fakeDoer{
		resp: map[string]*http.Response{
			libraryURL: {
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBuffer(b)),
			},
		},
	}
	names, err := scrapeLibrary(context.Background(), f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 5 {
		t.Fatalf("got %d names, want 5", len(names))
	}
	want := map[string]bool{"llama3": true, "llama3.1": true, "mistral": true, "qwen2.5": true, "gemma3": true}
	for _, n := range names {
		if !want[n] {
			t.Fatalf("unexpected name: %q", n)
		}
	}
}

func TestScrapeModelWithFixture(t *testing.T) {
	b, err := testdataFS.ReadFile("testdata/model.html.golden")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	f := &fakeDoer{
		resp: map[string]*http.Response{
			libraryURL + "/llama3.1": {
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBuffer(b)),
			},
		},
	}
	models, err := scrapeModel(context.Background(), f, "llama3.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 3 {
		t.Fatalf("got %d models, want 3", len(models))
	}
	found := map[string]float64{}
	for _, m := range models {
		found[m.Name] = m.SizeGB
	}
	if found["llama3.1:8b"] != 4.9 {
		t.Fatalf("llama3.1:8b size = %v, want 4.9", found["llama3.1:8b"])
	}
	if found["llama3.1:70b"] != 43.0 {
		t.Fatalf("llama3.1:70b size = %v, want 43.0", found["llama3.1:70b"])
	}
	if found["llama3.1:405b"] != 243.0 {
		t.Fatalf("llama3.1:405b size = %v, want 243.0", found["llama3.1:405b"])
	}
}

func TestScrapeModelSkipsLatest(t *testing.T) {
	f := &fakeDoer{
		resp: map[string]*http.Response{
			libraryURL + "/test": {
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`<html><body><a href="/library/test:latest">test:latest</a><a href="/library/test:7b">test:7b 4.1GB</a></body></html>`)),
			},
		},
	}
	models, err := scrapeModel(context.Background(), f, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("got %d models, want 1", len(models))
	}
	if models[0].Name != "test:7b" {
		t.Fatalf("Name = %q, want test:7b", models[0].Name)
	}
}

func TestScrapeAllWithFakeDoer(t *testing.T) {
	lib, _ := testdataFS.ReadFile("testdata/library.html.golden")
	mod, _ := testdataFS.ReadFile("testdata/model.html.golden")
	f := &fakeDoer{
		resp: map[string]*http.Response{
			libraryURL: {
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBuffer(lib)),
			},
			libraryURL + "/llama3": {
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBuffer(mod)),
			},
			libraryURL + "/llama3.1": {
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBuffer(mod)),
			},
			libraryURL + "/mistral": {
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBuffer(mod)),
			},
			libraryURL + "/qwen2.5": {
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBuffer(mod)),
			},
			libraryURL + "/gemma3": {
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBuffer(mod)),
			},
		},
	}
	models, err := scrapeAll(context.Background(), f, func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) == 0 {
		t.Fatalf("expected models, got none")
	}
}

func TestGetDocHTTPError(t *testing.T) {
	f := &fakeDoer{
		err: map[string]error{
			libraryURL: errors.New("network error"),
		},
	}
	_, err := getDoc(context.Background(), f, libraryURL)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestGetDocNon200(t *testing.T) {
	f := &fakeDoer{
		resp: map[string]*http.Response{
			libraryURL: {
				StatusCode: 500,
				Body:       io.NopCloser(bytes.NewBufferString("error")),
			},
		},
	}
	_, err := getDoc(context.Background(), f, libraryURL)
	if err == nil {
		t.Fatalf("expected error for non-200, got nil")
	}
}

func TestScrapeLibraryEmptyPage(t *testing.T) {
	f := &fakeDoer{
		resp: map[string]*http.Response{
			libraryURL: {
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`<html><body></body></html>`)),
			},
		},
	}
	names, err := scrapeLibrary(context.Background(), f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("got %d names, want 0", len(names))
	}
}

func TestScrapeModelEmptyPage(t *testing.T) {
	f := &fakeDoer{
		resp: map[string]*http.Response{
			libraryURL + "/empty": {
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`<html><body></body></html>`)),
			},
		},
	}
	models, err := scrapeModel(context.Background(), f, "empty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 0 {
		t.Fatalf("got %d models, want 0", len(models))
	}
}

func TestScrapeAllEmptyLibrary(t *testing.T) {
	f := &fakeDoer{
		resp: map[string]*http.Response{
			libraryURL: {
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`<html><body></body></html>`)),
			},
		},
	}
	_, err := scrapeAll(context.Background(), f, func(string) {})
	if err == nil {
		t.Fatalf("expected error for empty library, got nil")
	}
}
