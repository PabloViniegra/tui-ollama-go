// Package hardware detecta CPU, RAM y GPU/VRAM de forma multiplataforma
// (macOS Apple Silicon / Intel, Linux y Windows).
package hardware

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// GPUKind es un tipo de cadena tipada para los tipos de GPU.
type GPUKind string

const (
	GPUKindNone   GPUKind = "none"
	GPUKindNVIDIA GPUKind = "nvidia"
	GPUKindAMD    GPUKind = "amd"
	GPUKindApple  GPUKind = "apple"
	GPUKindIntel  GPUKind = "intel"
)

// GPU describe el acelerador detectado.
type GPU struct {
	Name   string  // nombre comercial (ej. "NVIDIA GeForce RTX 4070")
	VRAMGB float64 // VRAM dedicada en GB; 0 si es desconocida o no hay GPU
	Kind   GPUKind // nvidia | amd | apple | intel | none
}

// Info agrupa toda la información del equipo.
type Info struct {
	OS           string
	Arch         string
	CPUModel     string
	CPUCores     int
	RAMGB        float64
	GPU          GPU
	AppleUnified bool // true en Apple Silicon (memoria unificada CPU/GPU)
}

// CommandRunner abstracts os/exec so detectors can be tested without real GPUs.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

// execRunner is the production adapter. It keeps the existing 4 s timeout and
// returns an empty stdout on error.
type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return "", fmt.Errorf("exec %s: %w", name, err)
	}
	return string(out), nil
}

// runWithRunner is a small helper for non-GPU callers that still need a command.
func runWithRunner(runner CommandRunner, name string, args ...string) (string, bool) {
	out, err := runner.Run(context.Background(), name, args...)
	if err != nil {
		return "", false
	}
	return out, true
}

// Detect inspecciona el equipo actual y devuelve su Info.
func Detect(ctx context.Context) Info {
	info := Info{OS: runtime.GOOS, Arch: runtime.GOARCH}

	if vm, err := mem.VirtualMemory(); err == nil && vm != nil {
		info.RAMGB = bytesToGB(vm.Total)
	}
	info.CPUCores = logicalCores()
	info.CPUModel = cpuModel()

	if ctx.Err() != nil {
		return info
	}

	runner := execRunner{}
	info.GPU = detectGPU(runner)

	if info.OS == "darwin" && info.Arch == "arm64" {
		info.AppleUnified = true
		if info.GPU.Kind == "" || info.GPU.Kind == GPUKindNone {
			info.GPU = GPU{Name: appleChip(runner), Kind: GPUKindApple}
		}
	}
	return info
}

func bytesToGB(b uint64) float64 { return float64(b) / (1024.0 * 1024.0 * 1024.0) }

func logicalCores() int {
	if n, err := cpu.Counts(true); err == nil && n > 0 {
		return n
	}
	return runtime.NumCPU()
}

func cpuModel() string {
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		if s := sysctl("machdep.cpu.brand_string"); s != "" {
			return s
		}
	}
	if ci, err := cpu.Info(); err == nil && len(ci) > 0 && strings.TrimSpace(ci[0].ModelName) != "" {
		return strings.TrimSpace(ci[0].ModelName)
	}
	return "CPU desconocida"
}

func sysctl(key string) string {
	if out, ok := runWithRunner(execRunner{}, "sysctl", "-n", key); ok {
		return strings.TrimSpace(out)
	}
	return ""
}

func appleChip(runner CommandRunner) string {
	if out, ok := runWithRunner(runner, "sysctl", "-n", "machdep.cpu.brand_string"); ok {
		return strings.TrimSpace(out)
	}
	return "Apple Silicon"
}

// Detector abstracts a single GPU-discovery strategy.
type Detector interface {
	Detect(runner CommandRunner) (GPU, bool)
}

type nvidiaDetector struct{}
type amdLinuxDetector struct{}
type macIntelDetector struct{}
type windowsDetector struct{}

var detectorChain = []struct {
	name  string
	match func() bool
	det   Detector
}{
	{"nvidia", func() bool { return true }, nvidiaDetector{}},
	{"amd-linux", func() bool { return runtime.GOOS == "linux" }, amdLinuxDetector{}},
	{"mac-intel", func() bool { return runtime.GOOS == "darwin" && runtime.GOARCH != "arm64" }, macIntelDetector{}},
	{"windows", func() bool { return runtime.GOOS == "windows" }, windowsDetector{}},
}

// detectGPU prueba detectores en orden de fiabilidad y cae a CPU si nada responde.
func detectGPU(runner CommandRunner) GPU {
	for _, d := range detectorChain {
		if !d.match() {
			continue
		}
		if g, ok := d.det.Detect(runner); ok {
			return g
		}
	}
	return GPU{Kind: GPUKindNone}
}

func (nvidiaDetector) Detect(runner CommandRunner) (GPU, bool) {
	out, err := runner.Run(context.Background(), "nvidia-smi",
		"--query-gpu=name,memory.total", "--format=csv,noheader,nounits")
	if err != nil {
		return GPU{}, false
	}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		mib, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			continue
		}
		return GPU{Name: name, VRAMGB: mib / 1024.0, Kind: GPUKindNVIDIA}, true
	}
	return GPU{}, false
}

func (macIntelDetector) Detect(runner CommandRunner) (GPU, bool) {
	out, err := runner.Run(context.Background(), "system_profiler", "-json", "SPDisplaysDataType")
	if err != nil {
		return GPU{Kind: GPUKindNone}, false
	}
	var data struct {
		Displays []struct {
			Name       string `json:"sppci_model"`
			VRAM       string `json:"spdisplays_vram"`
			VRAMShared string `json:"spdisplays_vram_shared"`
		} `json:"SPDisplaysDataType"`
	}
	if json.Unmarshal([]byte(out), &data) != nil || len(data.Displays) == 0 {
		return GPU{Kind: GPUKindNone}, false
	}
	d := data.Displays[0]
	v := parseAppleVRAM(d.VRAM)
	if v == 0 {
		v = parseAppleVRAM(d.VRAMShared)
	}
	kind := GPUKindIntel
	low := strings.ToLower(d.Name)
	if strings.Contains(low, "amd") || strings.Contains(low, "radeon") {
		kind = GPUKindAMD
	}
	return GPU{Name: d.Name, VRAMGB: v, Kind: kind}, true
}

// parseAppleVRAM admite formatos tipo "1536 MB" o "8 GB".
func parseAppleVRAM(s string) float64 {
	f := strings.Fields(strings.TrimSpace(s))
	if len(f) == 0 {
		return 0
	}
	val, err := strconv.ParseFloat(f[0], 64)
	if err != nil {
		return 0
	}
	if len(f) >= 2 && strings.EqualFold(f[1], "MB") {
		return val / 1024.0
	}
	return val // se asume GB
}

func (amdLinuxDetector) Detect(runner CommandRunner) (GPU, bool) {
	out, err := runner.Run(context.Background(), "rocm-smi", "--showmeminfo", "vram", "--json")
	if err != nil {
		return GPU{}, false
	}
	var devices map[string]map[string]string
	if json.Unmarshal([]byte(out), &devices) != nil {
		return GPU{}, false
	}
	for _, dev := range devices {
		for k, v := range dev {
			if strings.Contains(strings.ToLower(k), "vram total memory") {
				if b, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
					return GPU{Name: "AMD GPU (ROCm)", VRAMGB: b / (1024 * 1024 * 1024), Kind: GPUKindAMD}, true
				}
			}
		}
	}
	return GPU{}, false
}

func (windowsDetector) Detect(runner CommandRunner) (GPU, bool) {
	ps := `Get-CimInstance Win32_VideoController | Select-Object Name,AdapterRAM | ConvertTo-Json`
	out, err := runner.Run(context.Background(), "powershell", "-NoProfile", "-Command", ps)
	if err != nil {
		return GPU{}, false
	}
	type vc struct {
		Name       string
		AdapterRAM int64
	}
	var one vc
	var many []vc
	if json.Unmarshal([]byte(out), &many) == nil && len(many) > 0 {
		one = many[0]
	} else if json.Unmarshal([]byte(out), &one) != nil {
		return GPU{}, false
	}
	low := strings.ToLower(one.Name)
	kind := GPUKindIntel
	switch {
	case strings.Contains(low, "nvidia"):
		kind = GPUKindNVIDIA
	case strings.Contains(low, "amd"), strings.Contains(low, "radeon"):
		kind = GPUKindAMD
	}
	// OJO: AdapterRAM es uint32 en WMI y satura en ~4 GB; puede quedar corto
	// en GPUs grandes. Por eso nvidia-smi se intenta antes.
	gb := float64(one.AdapterRAM) / (1024 * 1024 * 1024)
	if gb <= 0 {
		return GPU{Name: one.Name, Kind: kind}, true // VRAM desconocida
	}
	return GPU{Name: one.Name, VRAMGB: gb, Kind: kind}, true
}
