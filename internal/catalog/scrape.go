package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	baseURL    = "https://ollama.com"
	libraryURL = baseURL + "/library"
	userAgent  = "ollama-fit/1.0 (+https://github.com/) catalog-scraper"
	cacheTTL   = 24 * time.Hour
	maxWorkers = 8
)

var (
	reSize  = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(gb|mb)`)
	reParam = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*([bm])(?:[-_:]|$|\b)`)
	reQuant = regexp.MustCompile(`(?i)(q\d[k_0-9a-z]*|fp16|bf16|f16)`)
)

type cacheFile struct {
	FetchedAt time.Time `json:"fetched_at"`
	Models    []Model   `json:"models"`
}

// HTTPDoer abstracts *http.Client for testability.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Fetch devuelve el catálogo. Por defecto: caché fresca -> red (scrape) -> caché
// vieja -> catálogo embebido. Con offline usa directamente el embebido; con
// refresh ignora la caché. progress recibe mensajes de avance (puede ser nil).
func Fetch(refresh, offline bool, progress func(string)) ([]Model, error) {
	if progress == nil {
		progress = func(string) {}
	}
	if offline {
		progress("Modo offline: usando catálogo embebido.")
		return Models(), nil
	}

	path := cachePath()
	if !refresh {
		if cf, ok := loadCache(path); ok && len(cf.Models) > 0 && time.Since(cf.FetchedAt) < cacheTTL {
			progress(fmt.Sprintf("Catálogo desde caché (%d variantes, %s).",
				len(cf.Models), humanAge(cf.FetchedAt)))
			return cf.Models, nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 25 * time.Second}
	models, err := scrapeAll(ctx, client, progress)
	if err != nil || len(models) == 0 {
		if cf, ok := loadCache(path); ok && len(cf.Models) > 0 {
			progress("Scrape falló; uso caché previa.")
			return cf.Models, err
		}
		progress("Scrape falló y no hay caché; uso catálogo embebido.")
		return Models(), err
	}

	if err := saveCache(path, models); err != nil {
		progress("Aviso: no pude guardar la caché: " + err.Error())
	}
	return models, nil
}

func scrapeAll(ctx context.Context, doer HTTPDoer, progress func(string)) ([]Model, error) {
	progress("Recuperando lista de modelos de ollama.com/library…")
	names, err := scrapeLibrary(ctx, doer)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no se encontraron modelos en la librería")
	}
	progress(fmt.Sprintf("%d modelos en la librería. Leyendo tamaños…", len(names)))

	var (
		mu   sync.Mutex
		all  []Model
		wg   sync.WaitGroup
		sem  = make(chan struct{}, maxWorkers)
		done int32
	)
	for _, name := range names {
		wg.Add(1)
		sem <- struct{}{}
		go func(name string) {
			defer wg.Done()
			defer func() { <-sem }()
			if ms, err := scrapeModel(ctx, doer, name); err == nil {
				mu.Lock()
				all = append(all, ms...)
				mu.Unlock()
			}
			if n := atomic.AddInt32(&done, 1); n%25 == 0 {
				progress(fmt.Sprintf("  %d/%d modelos procesados…", n, len(names)))
			}
		}(name)
	}
	wg.Wait()

	sort.SliceStable(all, func(i, j int) bool { return all[i].Name < all[j].Name })
	progress(fmt.Sprintf("Catálogo recuperado: %d variantes.", len(all)))
	return all, nil
}

// scrapeLibrary devuelve los nombres base de modelo (sin tag) de /library.
func scrapeLibrary(ctx context.Context, doer HTTPDoer) ([]string, error) {
	doc, err := getDoc(ctx, doer, libraryURL)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var names []string
	doc.Find("a[href^='/library/']").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		rest := strings.TrimPrefix(href, "/library/")
		rest = strings.Trim(rest, "/")
		// solo páginas de modelo: sin '/', sin ':', no vacío
		if rest == "" || strings.ContainsAny(rest, "/:") {
			return
		}
		if !seen[rest] {
			seen[rest] = true
			names = append(names, rest)
		}
	})
	return names, nil
}

// scrapeModel lee /library/<name> y extrae sus variantes con tamaño.
func scrapeModel(ctx context.Context, doer HTTPDoer, name string) ([]Model, error) {
	doc, err := getDoc(ctx, doer, libraryURL+"/"+name)
	if err != nil {
		return nil, err
	}
	prefix := "/library/" + name + ":"
	// Para cada tag puede haber varios <a> con el mismo href; nos quedamos con
	// el texto más largo (el que incluye el tamaño "4.9GB · 128K · Text").
	best := map[string]string{}
	doc.Find("a[href^='/library/']").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		if !strings.HasPrefix(href, prefix) {
			return
		}
		tag := strings.TrimPrefix(href, "/library/")
		if strings.HasSuffix(tag, ":latest") { // alias; evita duplicar
			return
		}
		txt := strings.Join(strings.Fields(s.Text()), " ")
		if len(txt) > len(best[tag]) {
			best[tag] = txt
		}
	})

	var out []Model
	for tag, txt := range best {
		size := parseSizeGB(txt)
		if size <= 0 {
			continue // sin tamaño no podemos clasificar
		}
		out = append(out, Model{
			Name:   tag,
			Family: name,
			Params: parseParams(tag),
			Quant:  parseQuant(tag),
			SizeGB: size,
		})
	}
	return out, nil
}

func getDoc(ctx context.Context, doer HTTPDoer, url string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html")
	resp, err := doer.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s -> HTTP %d", url, resp.StatusCode)
	}
	return goquery.NewDocumentFromReader(resp.Body)
}

func parseSizeGB(s string) float64 {
	m := reSize.FindStringSubmatch(s)
	if len(m) < 3 {
		return 0
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	if strings.EqualFold(m[2], "MB") {
		return v / 1024.0
	}
	return v
}

func parseParams(tag string) string {
	// tag tipo "llama3.1:8b" o "gemma3:270m"
	part := tag
	if i := strings.Index(tag, ":"); i >= 0 {
		part = tag[i+1:]
	}
	if m := reParam.FindStringSubmatch(part); len(m) >= 3 {
		return m[1] + strings.ToUpper(m[2])
	}
	return "—"
}

func parseQuant(tag string) string {
	if m := reQuant.FindString(tag); m != "" {
		return strings.ToUpper(m)
	}
	return "default"
}

// ---------- caché ----------

func cachePath() string {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "ollama-fit", "catalog.json")
}

func loadCache(path string) (cacheFile, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return cacheFile{}, false
	}
	var cf cacheFile
	if json.Unmarshal(b, &cf) != nil {
		return cacheFile{}, false
	}
	return cf, true
}

func saveCache(path string, models []Model) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cacheFile{FetchedAt: time.Now(), Models: models}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func humanAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "hace segundos"
	case d < time.Hour:
		return fmt.Sprintf("hace %d min", int(d.Minutes()))
	default:
		return fmt.Sprintf("hace %d h", int(d.Hours()))
	}
}
