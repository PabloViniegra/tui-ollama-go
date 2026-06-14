// Package hardware detecta CPU, RAM y GPU/VRAM de forma multiplataforma
// (macOS Apple Silicon / Intel, Linux y Windows).
package hardware

import (
	"bufio"
	"context"
	"encoding/json"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// GPU describe el acelerador detectado.
type GPU struct {
	Name   string  // nombre comercial (ej. "NVIDIA GeForce RTX 4070")
	VRAMGB float64 // VRAM dedicada en GB; 0 si es desconocida o no hay GPU
	Kind   string  // nvidia | amd | apple | intel | none
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

// Detect inspecciona el equipo actual y devuelve su Info.
func Detect() Info {
	info := Info{OS: runtime.GOOS, Arch: runtime.GOARCH}

	if vm, err := mem.VirtualMemory(); err == nil && vm != nil {
		info.RAMGB = bytesToGB(vm.Total)
	}
	info.CPUCores = logicalCores()
	info.CPUModel = cpuModel()
	info.GPU = detectGPU()

	if info.OS == "darwin" && info.Arch == "arm64" {
		info.AppleUnified = true
		if info.GPU.Kind == "" || info.GPU.Kind == "none" {
			info.GPU = GPU{Name: appleChip(), Kind: "apple"}
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

// run ejecuta un comando con timeout y devuelve su stdout.
func run(name string, args ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

func sysctl(key string) string {
	if out, ok := run("sysctl", "-n", key); ok {
		return strings.TrimSpace(out)
	}
	return ""
}

func appleChip() string {
	if s := sysctl("machdep.cpu.brand_string"); s != "" {
		return s
	}
	return "Apple Silicon"
}

// detectGPU prueba detectores en orden de fiabilidad y cae a CPU si nada responde.
func detectGPU() GPU {
	// nvidia-smi funciona en Linux y Windows (y eGPU). Es lo más fiable.
	if g, ok := detectNVIDIA(); ok {
		return g
	}
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return GPU{Name: appleChip(), Kind: "apple"}
		}
		return detectMacIntelGPU()
	case "linux":
		if g, ok := detectAMDLinux(); ok {
			return g
		}
	case "windows":
		if g, ok := detectWindowsGPU(); ok {
			return g
		}
	}
	return GPU{Kind: "none"}
}

func detectNVIDIA() (GPU, bool) {
	out, ok := run("nvidia-smi",
		"--query-gpu=name,memory.total", "--format=csv,noheader,nounits")
	if !ok {
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
		return GPU{Name: name, VRAMGB: mib / 1024.0, Kind: "nvidia"}, true
	}
	return GPU{}, false
}

func detectMacIntelGPU() GPU {
	out, ok := run("system_profiler", "-json", "SPDisplaysDataType")
	if !ok {
		return GPU{Kind: "none"}
	}
	var data struct {
		Displays []struct {
			Name       string `json:"sppci_model"`
			VRAM       string `json:"spdisplays_vram"`
			VRAMShared string `json:"spdisplays_vram_shared"`
		} `json:"SPDisplaysDataType"`
	}
	if json.Unmarshal([]byte(out), &data) != nil || len(data.Displays) == 0 {
		return GPU{Kind: "none"}
	}
	d := data.Displays[0]
	v := parseAppleVRAM(d.VRAM)
	if v == 0 {
		v = parseAppleVRAM(d.VRAMShared)
	}
	kind := "intel"
	low := strings.ToLower(d.Name)
	if strings.Contains(low, "amd") || strings.Contains(low, "radeon") {
		kind = "amd"
	}
	return GPU{Name: d.Name, VRAMGB: v, Kind: kind}
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

func detectAMDLinux() (GPU, bool) {
	out, ok := run("rocm-smi", "--showmeminfo", "vram", "--json")
	if !ok {
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
					return GPU{Name: "AMD GPU (ROCm)", VRAMGB: b / (1024 * 1024 * 1024), Kind: "amd"}, true
				}
			}
		}
	}
	return GPU{}, false
}

func detectWindowsGPU() (GPU, bool) {
	ps := `Get-CimInstance Win32_VideoController | Select-Object Name,AdapterRAM | ConvertTo-Json`
	out, ok := run("powershell", "-NoProfile", "-Command", ps)
	if !ok {
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
	kind := "intel"
	switch {
	case strings.Contains(low, "nvidia"):
		kind = "nvidia"
	case strings.Contains(low, "amd"), strings.Contains(low, "radeon"):
		kind = "amd"
	}
	// OJO: AdapterRAM es uint32 en WMI y satura en ~4 GB; puede quedar corto
	// en GPUs grandes. Por eso nvidia-smi se intenta antes.
	gb := float64(one.AdapterRAM) / (1024 * 1024 * 1024)
	if gb <= 0 {
		return GPU{Name: one.Name, Kind: kind}, true // VRAM desconocida
	}
	return GPU{Name: one.Name, VRAMGB: gb, Kind: kind}, true
}
