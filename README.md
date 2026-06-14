# Ollama Fit

TUI en Go que detecta el hardware de tu equipo (CPU, RAM y GPU/VRAM) y te dice
qué modelos de Ollama corren bien, cuáles van justos y cuáles no caben.

```
 Ollama Fit   darwin/arm64 · 49 modelos   ●23 ●12 ●14
CPU  Apple M3 Pro · 12 núcleos    RAM  36.0 GB
GPU  Apple M3 Pro · memoria unificada (~25 GB usables por GPU)

 ESTADO   MODELO              PARÁM  CUANT     TAMAÑO   NECESITA   BACKEND
 Va bien  llama3.1:8b         8B     Q4_K_M    4.9 GB   5.9 GB     GPU Metal
 Va bien  qwen2.5:14b         14B    Q4_K_M    9.0 GB   10.8 GB    GPU Metal
 Justo    qwen2.5:32b         32B    Q4_K_M    20.0 GB  24.0 GB    GPU Metal
 No cabe  llama3.1:70b        70B    Q4_K_M    43.0 GB  51.6 GB    —
```

## Requisitos

- Go 1.22 o superior.
- (Opcional) GPU NVIDIA con `nvidia-smi` en el PATH para leer VRAM exacta.

## Instalar con `go install`

```bash
go install github.com/PabloViniegra/tui-ollama-go@latest
```

El binario se instala como `tui-ollama-go` en `$GOPATH/bin`. Para usarlo como `ollama-fit`, renombralo o crea un alias:

```bash
mv $(go env GOPATH)/bin/tui-ollama-go $(go env GOPATH)/bin/ollama-fit
```

> Nota: la ruta de caché sigue siendo `ollama-fit` (`os.UserCacheDir()/ollama-fit/catalog.json`) para mantener compatibilidad con instalaciones previas.

## Construir y ejecutar

```bash
cd ollama-fit
go mod tidy        # descarga dependencias (incluido goquery) y genera go.sum
go run .           # primera vez: scrapea ollama.com (unos segundos); luego cachea
# o: go build -o ollama-fit . && ./ollama-fit
```

> Nota: el proyecto se entregó sin compilar (el entorno donde se generó no tenía
> red para descargar los módulos). `go mod tidy` resolverá las dependencias.

## Controles

| Tecla            | Acción                                  |
|------------------|-----------------------------------------|
| `↑`/`↓` o `k`/`j`| moverse                                 |
| `pgup`/`pgdn`    | página arriba/abajo                     |
| `g` / `G`        | inicio / fin                            |
| `f`              | ciclar filtro (todos/va bien/justo/no)  |
| `/`              | buscar por nombre (enter aplica, esc limpia) |
| `q` / `esc`      | salir                                   |

## Cómo se decide el veredicto

Heurística RAM/VRAM-vs-tamaño (ajustable en `internal/eval/eval.go`):

1. Se estima la memoria necesaria: `necesita = tamaño_del_modelo × 1.2`
   (margen para KV-cache, contexto y runtime).
2. Se calcula la memoria "acelerada" disponible:
   - GPU dedicada → VRAM detectada.
   - Apple Silicon → `0.70 × RAM` (memoria unificada para Metal).
   - Sin GPU → solo RAM del sistema (CPU).
3. Veredicto:
   - **Va bien** — cabe con holgura (`necesita ≤ 0.85 × VRAM`), o cabe en RAM y
     es pequeño en CPU.
   - **Justo** — cabe pero apretado, con reparto GPU/CPU, o lento en CPU.
   - **No cabe** — supera la memoria disponible.

Los umbrales (`overhead`, `appleGPUFraction`, `0.85`, `0.90`) son constantes que
puedes tocar a tu gusto.

## Detección de hardware

- **RAM y CPU**: `gopsutil` (multiplataforma).
- **GPU**:
  - NVIDIA → `nvidia-smi` (Linux y Windows) — VRAM exacta.
  - Apple Silicon → memoria unificada (`sysctl`).
  - Mac Intel → `system_profiler`.
  - AMD Linux → `rocm-smi` (best-effort).
  - Windows (no-NVIDIA) → WMI/`Win32_VideoController` (best-effort; `AdapterRAM`
    satura en ~4 GB, puede quedarse corto en GPUs grandes).
  - Si nada responde → CPU.

## Catálogo (scrapeado de ollama.com)

El catálogo se obtiene **en vivo desde la propia web de Ollama**, que es HTML
renderizado en servidor (Ollama no expone API pública para navegar la librería):

1. `https://ollama.com/library` → lista de modelos.
2. `https://ollama.com/library/<modelo>` → variantes de cada modelo con su
   tamaño real en GB (ej. `llama3.1:8b … 4.9GB · 128K context window`).

Se parsea con `goquery` y se guarda en **caché local** (`os.UserCacheDir()/ollama-fit/catalog.json`)
durante 24 h, así que la primera ejecución tarda unos segundos y las siguientes
son instantáneas.

Orden de resolución: caché fresca → scrape en vivo → caché vieja → catálogo
embebido de respaldo (`internal/catalog/catalog.go`, por si no hay red).

Flags:

```bash
go run . --refresh   # ignora la caché y vuelve a scrapear
go run . --offline   # no toca la red; usa el catálogo embebido
```

> El scrape lee la página de cada modelo (las variantes principales: latest, 8b,
> 70b, …). Para no martillear ollama.com usa concurrencia limitada (8) y caché.
> Si quisieras **todas** las cuantizaciones de cada modelo, habría que leer
> además `…/library/<modelo>/tags`; está fuera del alcance actual para mantener
> el scrape ligero.

### Veredicto one-shot (sin TUI)

Para chequear un modelo sin abrir la interfaz, está el subcomando `fit`:

```bash
ollama-fit fit qwen2.5:7b
```

Imprime veredicto, backend, motivo y memoria necesaria vs. disponible. Códigos
de salida: `0` Good, `1` Tight, `2` No, `3` Error. Útil para scripting tipo
`ollama-fit fit X && ollama run X`.

Para que funcione "global" desde cualquier directorio, instalá el binario con
`make install` (que ejecuta `go install .`) y asegurate de que `$GOPATH/bin`
(o `$HOME/go/bin`) esté en tu `PATH`.

### Ideas para iterar

- Leer también `…/tags` para todas las cuantizaciones (q4_K_M, q8_0, fp16…).
- **Modelos locales**: añadir un modo que lea `ollama list` y marque lo instalado.
- **Benchmark real**: lanzar un prompt corto contra `ollama run` y medir tokens/s.

## Estructura

```
ollama-fit/
├── go.mod
├── main.go                     # detecta hardware, obtiene catálogo, evalúa y lanza la TUI
└── internal/
    ├── hardware/hardware.go    # detección CPU/RAM/GPU multiplataforma
    ├── catalog/scrape.go       # scraper en vivo de ollama.com + caché
    ├── catalog/catalog.go      # catálogo embebido de respaldo (offline)
    ├── eval/eval.go            # heurística de clasificación
    └── tui/tui.go              # interfaz Bubble Tea
```
