package eval

import (
	"testing"

	"ollama-fit/internal/catalog"
	"ollama-fit/internal/hardware"
)

func TestGpuLabel(t *testing.T) {
	tests := []struct {
		kind hardware.GPUKind
		want string
	}{
		{hardware.GPUKindNVIDIA, "CUDA"},
		{hardware.GPUKindAMD, "ROCm"},
		{hardware.GPUKindApple, "Metal"},
		{hardware.GPUKindIntel, "iGPU"},
		{hardware.GPUKindNone, ""},
		{"unknown", ""},
		{"", ""},
	}
	for _, tc := range tests {
		t.Run(string(tc.kind), func(t *testing.T) {
			got := gpuLabel(tc.kind)
			if got != tc.want {
				t.Errorf("gpuLabel(%q) = %q, want %q", tc.kind, got, tc.want)
			}
		})
	}
}

func model(name string, sizeGB float64) catalog.Model {
	return catalog.Model{Name: name, Family: name, Params: "7B", Quant: "Q4_K_M", SizeGB: sizeGB}
}

func nvidia(vramGB float64) hardware.Info {
	return hardware.Info{
		RAMGB: 32,
		GPU:   hardware.GPU{Name: "NVIDIA RTX 4070", VRAMGB: vramGB, Kind: hardware.GPUKindNVIDIA},
	}
}

func noGPU(ramGB float64) hardware.Info {
	return hardware.Info{RAMGB: ramGB}
}

func appleSilicon(ramGB float64) hardware.Info {
	return hardware.Info{
		RAMGB:        ramGB,
		AppleUnified: true,
		GPU:          hardware.GPU{Name: "M2 Pro", Kind: hardware.GPUKindApple},
	}
}

func TestEvaluateGoodInGPU(t *testing.T) {
	// need = 4 * 1.2 = 4.8 GB; 0.85 * 16 = 13.6 -> Good
	r := Evaluate(nvidia(16), model("small", 4))
	if r.Verdict != Good {
		t.Errorf("verdict = %v, want Good", r.Verdict)
	}
	if r.Backend != "GPU CUDA" {
		t.Errorf("backend = %q, want %q", r.Backend, "GPU CUDA")
	}
}

func TestEvaluateTightInGPU(t *testing.T) {
	// need = 8 * 1.2 = 9.6 GB; 0.85*10=8.5 < 9.6 <= 10 -> Tight in GPU
	r := Evaluate(nvidia(10), model("medium", 8))
	if r.Verdict != Tight {
		t.Errorf("verdict = %v, want Tight", r.Verdict)
	}
	if r.Backend != "GPU CUDA" {
		t.Errorf("backend = %q, want %q", r.Backend, "GPU CUDA")
	}
}

func TestEvaluateGPUCPUOffload(t *testing.T) {
	// need = 8 * 1.2 = 9.6; doesn't fit in 8GB VRAM; 9.6 <= 0.70*32=22.4 -> Tight GPU+CPU
	r := Evaluate(nvidia(8), model("medium", 8))
	if r.Verdict != Tight {
		t.Errorf("verdict = %v, want Tight", r.Verdict)
	}
	if r.Backend != "GPU+CPU" {
		t.Errorf("backend = %q, want %q", r.Backend, "GPU+CPU")
	}
}

func TestEvaluateGoodCPUSmall(t *testing.T) {
	// no GPU; need = 4 * 1.2 = 4.8 <= 0.70*16=11.2; need <= 6 -> Good CPU
	r := Evaluate(noGPU(16), model("small", 4))
	if r.Verdict != Good {
		t.Errorf("verdict = %v, want Good", r.Verdict)
	}
	if r.Backend != "CPU" {
		t.Errorf("backend = %q, want %q", r.Backend, "CPU")
	}
}

func TestEvaluateTightCPULarge(t *testing.T) {
	// no GPU; need = 6.5 * 1.2 = 7.8 <= 0.70*16=11.2; need > 6 -> Tight CPU
	r := Evaluate(noGPU(16), model("large", 6.5))
	if r.Verdict != Tight {
		t.Errorf("verdict = %v, want Tight", r.Verdict)
	}
	if r.Backend != "CPU" {
		t.Errorf("backend = %q, want %q", r.Backend, "CPU")
	}
}

func TestEvaluateTightRAMPressure(t *testing.T) {
	// no GPU; need = 11 * 1.2 = 13.2; > 0.70*16=11.2; <= 0.90*16=14.4 -> Tight "Apura la RAM"
	r := Evaluate(noGPU(16), model("xlarge", 11))
	if r.Verdict != Tight {
		t.Errorf("verdict = %v, want Tight", r.Verdict)
	}
	if r.Backend != "CPU" {
		t.Errorf("backend = %q, want %q", r.Backend, "CPU")
	}
}

func TestEvaluateNoFit(t *testing.T) {
	// no GPU; need = 14 * 1.2 = 16.8; > 0.90*16=14.4 -> No
	r := Evaluate(noGPU(16), model("giant", 14))
	if r.Verdict != No {
		t.Errorf("verdict = %v, want No", r.Verdict)
	}
	if r.Backend != "—" {
		t.Errorf("backend = %q, want %q", r.Backend, "—")
	}
}

func TestEvaluateAppleSiliconGood(t *testing.T) {
	// apple; gpuMem = 0.70*24=16.8; need = 4*1.2=4.8 <= 0.85*16.8=14.28 -> Good Metal
	r := Evaluate(appleSilicon(24), model("small", 4))
	if r.Verdict != Good {
		t.Errorf("verdict = %v, want Good", r.Verdict)
	}
	if r.Backend != "GPU Metal" {
		t.Errorf("backend = %q, want %q", r.Backend, "GPU Metal")
	}
}

func TestEvaluateAppleSiliconTight(t *testing.T) {
	// apple; gpuMem = 0.70*16=11.2; need = 9*1.2=10.8; 0.85*11.2=9.52 < 10.8 <= 11.2 -> Tight Metal
	r := Evaluate(appleSilicon(16), model("medium", 9))
	if r.Verdict != Tight {
		t.Errorf("verdict = %v, want Tight", r.Verdict)
	}
	if r.Backend != "GPU Metal" {
		t.Errorf("backend = %q, want %q", r.Backend, "GPU Metal")
	}
}

func TestEvaluateNeedGB(t *testing.T) {
	m := model("test", 5)
	r := Evaluate(noGPU(32), m)
	want := 5 * overhead
	if r.NeedGB != want {
		t.Errorf("NeedGB = %v, want %v", r.NeedGB, want)
	}
}

func TestEvaluateModelPreserved(t *testing.T) {
	m := model("llama3.1:8b", 4.9)
	r := Evaluate(noGPU(32), m)
	if r.Model.Name != "llama3.1:8b" {
		t.Errorf("Model.Name = %q, want %q", r.Model.Name, "llama3.1:8b")
	}
}
