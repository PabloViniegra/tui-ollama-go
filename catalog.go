// Package catalog obtiene el catálogo de modelos de Ollama. La fuente principal
// es el scraping en vivo de ollama.com (ver scrape.go); esta lista embebida es
// solo el RESPALDO OFFLINE que se usa si no hay red ni caché. Sus tamaños son
// aproximados y pueden quedar desfasados respecto a la web.
package catalog

// Model representa una etiqueta concreta de un modelo (familia:tag).
type Model struct {
	Name   string  `json:"name"`   // ej. "llama3.1:8b"
	Family string  `json:"family"` // ej. "llama3.1"
	Params string  `json:"params"` // ej. "8B"
	Quant  string  `json:"quant"`  // ej. "Q4_K_M" o "default"
	SizeGB float64 `json:"size_gb"`
}

// Models devuelve el catálogo embebido de respaldo (offline).
func Models() []Model {
	return []Model{
		// --- Edge / muy pequeños ---
		{"qwen2.5:0.5b", "qwen2.5", "0.5B", "Q4_K_M", 0.4},
		{"tinyllama:1.1b", "tinyllama", "1.1B", "Q4_0", 0.6},
		{"gemma3:1b", "gemma3", "1B", "Q4_K_M", 0.8},
		{"qwen2.5:1.5b", "qwen2.5", "1.5B", "Q4_K_M", 1.0},
		{"deepseek-r1:1.5b", "deepseek-r1", "1.5B", "Q4_K_M", 1.1},
		{"llama3.2:1b", "llama3.2", "1B", "Q4_K_M", 1.3},
		{"gemma2:2b", "gemma2", "2B", "Q4_K_M", 1.6},
		{"qwen2.5:3b", "qwen2.5", "3B", "Q4_K_M", 1.9},
		{"llama3.2:3b", "llama3.2", "3B", "Q4_K_M", 2.0},
		{"phi3:3.8b", "phi3", "3.8B", "Q4_0", 2.2},
		{"gemma3:4b", "gemma3", "4B", "Q4_K_M", 3.3},

		// --- Gama media (7-9B) ---
		{"codellama:7b", "codellama", "7B", "Q4_0", 3.8},
		{"mistral:7b", "mistral", "7B", "Q4_0", 4.1},
		{"llama3:8b", "llama3", "8B", "Q4_0", 4.7},
		{"qwen2.5:7b", "qwen2.5", "7B", "Q4_K_M", 4.7},
		{"qwen2.5-coder:7b", "qwen2.5-coder", "7B", "Q4_K_M", 4.7},
		{"llava:7b", "llava", "7B", "Q4_0", 4.7},
		{"llama3.1:8b", "llama3.1", "8B", "Q4_K_M", 4.9},
		{"deepseek-r1:8b", "deepseek-r1", "8B", "Q4_K_M", 4.9},
		{"gemma2:9b", "gemma2", "9B", "Q4_K_M", 5.4},

		// --- Media-alta (12-16B) ---
		{"mistral-nemo:12b", "mistral-nemo", "12B", "Q4_K_M", 7.1},
		{"codellama:13b", "codellama", "13B", "Q4_0", 7.4},
		{"phi3:14b", "phi3", "14B", "Q4_0", 7.9},
		{"llava:13b", "llava", "13B", "Q4_0", 8.0},
		{"gemma3:12b", "gemma3", "12B", "Q4_K_M", 8.1},
		{"deepseek-coder-v2:16b", "deepseek-coder-v2", "16B", "Q4_K_M", 8.9},
		{"qwen2.5:14b", "qwen2.5", "14B", "Q4_K_M", 9.0},
		{"deepseek-r1:14b", "deepseek-r1", "14B", "Q4_K_M", 9.0},
		{"phi4:14b", "phi4", "14B", "Q4_K_M", 9.1},

		// --- Grandes (27-47B / MoE) ---
		{"gemma2:27b", "gemma2", "27B", "Q4_K_M", 16},
		{"gemma3:27b", "gemma3", "27B", "Q4_K_M", 17},
		{"codellama:34b", "codellama", "34B", "Q4_0", 19},
		{"qwen2.5:32b", "qwen2.5", "32B", "Q4_K_M", 20},
		{"qwen2.5-coder:32b", "qwen2.5-coder", "32B", "Q4_K_M", 20},
		{"deepseek-r1:32b", "deepseek-r1", "32B", "Q4_K_M", 20},
		{"command-r:35b", "command-r", "35B", "Q4_K_M", 20},
		{"llava:34b", "llava", "34B", "Q4_0", 20},
		{"mixtral:8x7b", "mixtral", "8x7B", "Q4_0", 26},

		// --- XL (70B+) ---
		{"codellama:70b", "codellama", "70B", "Q4_0", 39},
		{"llama3:70b", "llama3", "70B", "Q4_0", 40},
		{"llama3.1:70b", "llama3.1", "70B", "Q4_K_M", 43},
		{"deepseek-r1:70b", "deepseek-r1", "70B", "Q4_K_M", 43},
		{"qwen2.5:72b", "qwen2.5", "72B", "Q4_K_M", 47},
		{"mixtral:8x22b", "mixtral", "8x22B", "Q4_0", 80},

		// --- Gigantes (solo servidores) ---
		{"llama3.1:405b", "llama3.1", "405B", "Q4_K_M", 243},
		{"deepseek-r1:671b", "deepseek-r1", "671B", "Q4_K_M", 404},

		// --- Embeddings ---
		{"nomic-embed-text", "nomic-embed-text", "137M", "F16", 0.27},
		{"mxbai-embed-large", "mxbai-embed-large", "335M", "F16", 0.67},
	}
}
