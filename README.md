# Ollama Fit

TUI en Go que detecta el hardware de tu equipo (CPU, RAM y GPU/VRAM) y te
indica qué modelos de Ollama funcionan bien, cuáles van justos y cuáles no
caben.

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

- Go 1.23 o superior.
- (Opcional) GPU NVIDIA con `nvidia-smi` en el `PATH` para leer la VRAM exacta.

## Instalación

El binario se llama `ollama-fit` en todos los métodos. Elige el que prefieras:

### Desde Go (cualquier plataforma con Go 1.23+)

```bash
go install github.com/PabloViniegra/tui-ollama-go@latest
```

`go install` no admite `-o`, así que el binario queda como `tui-ollama-go` en
`$GOPATH/bin`. Para usarlo como `ollama-fit`:

```bash
mv $(go env GOPATH)/bin/tui-ollama-go $(go env GOPATH)/bin/ollama-fit
```

### macOS / Linux con Homebrew

```bash
brew tap PabloViniegra/ollama-fit
brew install ollama-fit
```

Actualización a versiones futuras: `brew upgrade ollama-fit`.

### Windows con Scoop

```powershell
scoop bucket add scoop-ollama-fit https://github.com/PabloViniegra/scoop-ollama-fit.git
scoop install scoop-ollama-fit/ollama-fit
```

Actualización a versiones futuras: `scoop update ollama-fit`.

### Binarios precompilados

Descarga el archivo para tu plataforma desde
[la última release](https://github.com/PabloViniegra/tui-ollama-go/releases/latest)
y ponlo en tu `PATH`. Hay binarios para Linux y macOS (amd64 y arm64) y
Windows (amd64), distribuidos como `.tar.gz` y `.zip` respectivamente.

> Nota: la ruta de caché sigue siendo `ollama-fit`
> (`os.UserCacheDir()/ollama-fit/catalog.json`) en todos los métodos, para
> mantener la compatibilidad con instalaciones previas.

## Construir y ejecutar

```bash
cd ollama-fit
go mod tidy        # descarga dependencias (incluido goquery) y genera go.sum
go run .           # primera vez: extrae el catálogo de ollama.com (unos segundos); luego usa caché
# o: go build -o ollama-fit . && ./ollama-fit
```

> Nota: el proyecto se entregó sin compilar (el entorno donde se generó no
> tenía red para descargar los módulos). `go mod tidy` resolverá las
> dependencias.

### Versión

Para consultar la versión del binario:

```bash
./ollama-fit --version    # o -V
# ollama-fit dev (linux/amd64, go1.23.2, commit=none, built=unknown)
```

En builds locales sin ldflags verás `dev`; los binarios publicados a través
de GoReleaser inyectan automáticamente la versión, el commit y la fecha
(véase `.goreleaser.yaml`).

## Controles

| Tecla            | Acción                                          |
|------------------|-------------------------------------------------|
| `↑`/`↓` o `k`/`j`| mover el cursor                                 |
| `pgup`/`pgdn`    | página arriba/abajo                             |
| `g` / `G`        | inicio / fin                                    |
| `f`              | ciclar el filtro (todos / va bien / justo / no) |
| `/`              | buscar por nombre (intro aplica, esc limpia)    |
| `q` / `esc`      | salir                                           |

## Cómo se decide el veredicto

Heurística de RAM/VRAM frente al tamaño (ajustable en
`internal/eval/eval.go`):

1. Se estima la memoria necesaria: `necesita = tamaño_del_modelo × 1.2`
   (margen para la KV-cache, el contexto y el runtime).
2. Se calcula la memoria "acelerada" disponible:
   - GPU dedicada → VRAM detectada.
   - Apple Silicon → `0.70 × RAM` (memoria unificada para Metal).
   - Sin GPU → solo RAM del sistema (CPU).
3. Veredicto:
   - **Va bien** — cabe con holgura (`necesita ≤ 0.85 × VRAM`), o cabe en
     RAM y es pequeño en CPU.
   - **Justo** — cabe pero apretado, con reparto GPU/CPU, o lento en CPU.
   - **No cabe** — supera la memoria disponible.

Los umbrales (`overhead`, `appleGPUFraction`, `0.85`, `0.90`) son constantes
que puedes modificar a tu gusto.

## Detección de hardware

- **RAM y CPU**: `gopsutil` (multiplataforma).
- **GPU**:
  - NVIDIA → `nvidia-smi` (Linux y Windows) — VRAM exacta.
  - Apple Silicon → memoria unificada (`sysctl`).
  - Mac Intel → `system_profiler`.
  - AMD Linux → `rocm-smi` (best-effort).
  - Windows (no NVIDIA) → WMI/`Win32_VideoController` (best-effort;
    `AdapterRAM` satura en ~4 GB, puede quedarse corto en GPUs grandes).
  - Si nada responde → CPU.

## Catálogo (extraído de ollama.com)

El catálogo se obtiene **en vivo desde la propia web de Ollama**, que
devuelve HTML renderizado en el servidor (Ollama no expone una API pública
para consultar el catálogo):

1. `https://ollama.com/library` → lista de modelos.
2. `https://ollama.com/library/<modelo>` → variantes de cada modelo con su
   tamaño real en GB (p. ej. `llama3.1:8b … 4.9GB · 128K context window`).

Se procesa con `goquery` y se guarda en **caché local**
(`os.UserCacheDir()/ollama-fit/catalog.json`) durante 24 h, de modo que la
primera ejecución tarda unos segundos y las siguientes son instantáneas.

Orden de resolución: caché reciente → extracción en vivo → caché antigua →
catálogo embebido de respaldo (`internal/catalog/catalog.go`, por si no hay
red).

Flags:

```bash
go run . --refresh   # ignora la caché y vuelve a extraer el catálogo
go run . --offline   # no accede a la red; usa el catálogo embebido
```

> La extracción lee la página de cada modelo (las variantes principales:
> latest, 8b, 70b, …). Para no saturar ollama.com usa concurrencia limitada
> (8) y caché. Si quisieras **todas** las cuantizaciones de cada modelo,
> habría que leer además `…/library/<modelo>/tags`; está fuera del alcance
> actual para mantener la extracción ligera.

### Veredicto one-shot (sin TUI)

Para comprobar un modelo sin abrir la interfaz, existe el subcomando `fit`:

```bash
ollama-fit fit qwen2.5:7b
```

Muestra el veredicto, el backend, el motivo y la memoria necesaria frente a
la disponible. Códigos de salida: `0` Good, `1` Tight, `2` No, `3` Error.
Útil para scripts del tipo `ollama-fit fit X && ollama run X`.

#### Salida estructurada para scripts (`--json`)

```bash
ollama-fit fit --json qwen2.5:7b
```

Emite un único objeto JSON por línea, fácil de consumir con `jq` desde
CI/scripts:

```bash
ollama-fit fit --json qwen2.5:7b | jq -e '.verdict == "good"'
```

El esquema es estable: `verdict`, `backend`, `need_gb`, `available_gb`,
`reason` y `model.{name,family,params,quant,size_gb}`. Los valores de
`verdict` van en minúsculas y son `"good"`, `"tight"` o `"no"`.

#### Traza detallada del cálculo (`--explain`)

```bash
ollama-fit fit --explain llama3.1:70b
```

Muestra, paso a paso, el modelo, su tamaño, la memoria necesaria con el
margen aplicado, la memoria acelerada disponible, el backend elegido y la
razón concreta del veredicto. Útil cuando el resultado te sorprende y
quieres entender por qué.

`--json` y `--explain` son mutuamente excluyentes.

Para que el binario funcione desde cualquier directorio, sigue la sección
[Instalación](#instalación) más arriba (el binario canónico se llama
`ollama-fit`) y asegúrate de que `$GOPATH/bin` (o `$HOME/go/bin`) esté en tu
`PATH`.

#### Diagnóstico del sistema (`doctor`)

Para auditar qué herramientas del sistema están disponibles y qué versión
tienen, sin abrir la TUI:

```bash
ollama-fit doctor
```

Imprime una tabla con las tools relevantes (`nvidia-smi`, `rocm-smi`,
`ollama`, `sysctl`, `uname`, `sw_vers`) y su estado (`ok` o `missing`). Sale
con código `0` si todas están presentes y `1` si falta alguna — útil como
smoke-test en CI:

```bash
ollama-fit doctor || echo "faltan herramientas en este runner"
```

### Ideas para iterar

- Leer también `…/tags` para obtener todas las cuantizaciones
  (q4_K_M, q8_0, fp16…).
- **Modelos locales**: añadir un modo que lea `ollama list` y marque lo
  instalado.
- **Benchmark real**: ejecutar un prompt corto en `ollama run` y medir
  tokens/s.

## Estructura

```
ollama-fit/
├── go.mod
├── main.go                     # detecta hardware, obtiene el catálogo, evalúa y lanza la TUI
└── internal/
    ├── hardware/hardware.go    # detección de CPU/RAM/GPU multiplataforma
    ├── catalog/scrape.go       # extracción en vivo de ollama.com + caché
    ├── catalog/catalog.go      # catálogo embebido de respaldo (offline)
    ├── eval/eval.go            # heurística de clasificación
    └── tui/tui.go              # interfaz Bubble Tea
```
