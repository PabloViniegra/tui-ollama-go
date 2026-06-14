package hardware

import (
	"context"
	"errors"
	"testing"
)

// cmdKey builds a lookup key for the fakeRunner.
func cmdKey(name string, args ...string) string {
	key := name
	for _, a := range args {
		key += " " + a
	}
	return key
}

// fakeRunnerV2 is a more flexible test double for CommandRunner.
type fakeRunnerV2 struct {
	output map[string]string
	err    map[string]error
}

func (f *fakeRunnerV2) Run(ctx context.Context, name string, args ...string) (string, error) {
	key := cmdKey(name, args...)
	if e, ok := f.err[key]; ok {
		return "", e
	}
	if out, ok := f.output[key]; ok {
		return out, nil
	}
	return "", errors.New("command not found: " + key)
}

func TestNvidiaDetectorHappyPath(t *testing.T) {
	f := &fakeRunnerV2{
		output: map[string]string{
			cmdKey("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits"): "NVIDIA GeForce RTX 4070, 12288",
		},
	}
	g, ok := nvidiaDetector{}.Detect(f)
	if !ok {
		t.Fatalf("expected detection to succeed")
	}
	if g.Name != "NVIDIA GeForce RTX 4070" {
		t.Fatalf("Name = %q, want %q", g.Name, "NVIDIA GeForce RTX 4070")
	}
	if g.VRAMGB != 12.0 {
		t.Fatalf("VRAMGB = %v, want 12.0", g.VRAMGB)
	}
	if g.Kind != GPUKindNVIDIA {
		(t.Fatalf)("Kind = %q, want %q", g.Kind, GPUKindNVIDIA)
	}
}

func TestNvidiaDetectorMultiLine(t *testing.T) {
	f := &fakeRunnerV2{
		output: map[string]string{
			cmdKey("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits"): "NVIDIA GeForce RTX 4090, 24576\nNVIDIA GeForce RTX 4070, 12288",
		},
	}
	g, ok := nvidiaDetector{}.Detect(f)
	if !ok {
		t.Fatalf("expected detection to succeed")
	}
	if g.Name != "NVIDIA GeForce RTX 4090" {
		t.Fatalf("Name = %q, want %q", g.Name, "NVIDIA GeForce RTX 4090")
	}
	if g.VRAMGB != 24.0 {
		t.Fatalf("VRAMGB = %v, want 24.0", g.VRAMGB)
	}
}

func TestNvidiaDetectorCommandNotFound(t *testing.T) {
	f := &fakeRunnerV2{
		err: map[string]error{
			cmdKey("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits"): errors.New("command not found"),
		},
	}
	_, ok := nvidiaDetector{}.Detect(f)
	if ok {
		t.Fatalf("expected detection to fail")
	}
}

func TestNvidiaDetectorMalformedOutput(t *testing.T) {
	f := &fakeRunnerV2{
		output: map[string]string{
			cmdKey("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits"): "bad output",
		},
	}
	_, ok := nvidiaDetector{}.Detect(f)
	if ok {
		t.Fatalf("expected detection to fail with malformed output")
	}
}

func TestNvidiaDetectorEmptyOutput(t *testing.T) {
	f := &fakeRunnerV2{
		output: map[string]string{
			cmdKey("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits"): "",
		},
	}
	_, ok := nvidiaDetector{}.Detect(f)
	if ok {
		t.Fatalf("expected detection to fail with empty output")
	}
}

func TestAmdLinuxDetectorHappyPath(t *testing.T) {
	f := &fakeRunnerV2{
		output: map[string]string{
			cmdKey("rocm-smi", "--showmeminfo", "vram", "--json"): `{"card0":{"VRAM Total Memory":"8589934592"}}`,
		},
	}
	g, ok := amdLinuxDetector{}.Detect(f)
	if !ok {
		t.Fatalf("expected detection to succeed")
	}
	if g.Name != "AMD GPU (ROCm)" {
		t.Fatalf("Name = %q, want %q", g.Name, "AMD GPU (ROCm)")
	}
	if g.VRAMGB != 8.0 {
		t.Fatalf("VRAMGB = %v, want 8.0", g.VRAMGB)
	}
	if g.Kind != GPUKindAMD {
		(t.Fatalf)("Kind = %q, want %q", g.Kind, GPUKindAMD)
	}
}

func TestAmdLinuxDetectorCommandNotFound(t *testing.T) {
	f := &fakeRunnerV2{
		err: map[string]error{
			cmdKey("rocm-smi", "--showmeminfo", "vram", "--json"): errors.New("command not found"),
		},
	}
	_, ok := amdLinuxDetector{}.Detect(f)
	if ok {
		t.Fatalf("expected detection to fail")
	}
}

func TestAmdLinuxDetectorMalformedJSON(t *testing.T) {
	f := &fakeRunnerV2{
		output: map[string]string{
			cmdKey("rocm-smi", "--showmeminfo", "vram", "--json"): "not json",
		},
	}
	_, ok := amdLinuxDetector{}.Detect(f)
	if ok {
		t.Fatalf("expected detection to fail with malformed JSON")
	}
}

func TestMacIntelDetectorHappyPath(t *testing.T) {
	f := &fakeRunnerV2{
		output: map[string]string{
			cmdKey("system_profiler", "-json", "SPDisplaysDataType"): `{"SPDisplaysDataType":[{"sppci_model":"Intel Iris Plus Graphics","spdisplays_vram":"1536 MB"}]}`,
		},
	}
	g, ok := macIntelDetector{}.Detect(f)
	if !ok {
		t.Fatalf("expected detection to succeed")
	}
	if g.Name != "Intel Iris Plus Graphics" {
		t.Fatalf("Name = %q, want %q", g.Name, "Intel Iris Plus Graphics")
	}
	if g.VRAMGB != 1.5 {
		t.Fatalf("VRAMGB = %v, want 1.5", g.VRAMGB)
	}
	if g.Kind != GPUKindIntel {
		(t.Fatalf)("Kind = %q, want %q", g.Kind, GPUKindIntel)
	}
}

func TestMacIntelDetectorAMDGPU(t *testing.T) {
	f := &fakeRunnerV2{
		output: map[string]string{
			cmdKey("system_profiler", "-json", "SPDisplaysDataType"): `{"SPDisplaysDataType":[{"sppci_model":"AMD Radeon Pro 5500M","spdisplays_vram":"4 GB"}]}`,
		},
	}
	g, ok := macIntelDetector{}.Detect(f)
	if !ok {
		t.Fatalf("expected detection to succeed")
	}
	if g.Kind != GPUKindAMD {
		(t.Fatalf)("Kind = %q, want %q", g.Kind, GPUKindAMD)
	}
}

func TestMacIntelDetectorFallbackVRAMShared(t *testing.T) {
	f := &fakeRunnerV2{
		output: map[string]string{
			cmdKey("system_profiler", "-json", "SPDisplaysDataType"): `{"SPDisplaysDataType":[{"sppci_model":"Intel UHD","spdisplays_vram":"","spdisplays_vram_shared":"2048 MB"}]}`,
		},
	}
	g, ok := macIntelDetector{}.Detect(f)
	if !ok {
		t.Fatalf("expected detection to succeed")
	}
	if g.VRAMGB != 2.0 {
		t.Fatalf("VRAMGB = %v, want 2.0", g.VRAMGB)
	}
}

func TestMacIntelDetectorCommandNotFound(t *testing.T) {
	f := &fakeRunnerV2{
		err: map[string]error{
			cmdKey("system_profiler", "-json", "SPDisplaysDataType"): errors.New("command not found"),
		},
	}
	_, ok := macIntelDetector{}.Detect(f)
	if ok {
		t.Fatalf("expected detection to fail")
	}
}

func TestMacIntelDetectorEmptyDisplays(t *testing.T) {
	f := &fakeRunnerV2{
		output: map[string]string{
			cmdKey("system_profiler", "-json", "SPDisplaysDataType"): `{"SPDisplaysDataType":[]}`,
		},
	}
	_, ok := macIntelDetector{}.Detect(f)
	if ok {
		t.Fatalf("expected detection to fail with empty displays")
	}
}

func TestWindowsDetectorHappyPath(t *testing.T) {
	f := &fakeRunnerV2{
		output: map[string]string{
			cmdKey("powershell", "-NoProfile", "-Command", `Get-CimInstance Win32_VideoController | Select-Object Name,AdapterRAM | ConvertTo-Json`): `[{"Name":"NVIDIA GeForce GTX 1660","AdapterRAM":6442450944}]`,
		},
	}
	g, ok := windowsDetector{}.Detect(f)
	if !ok {
		t.Fatalf("expected detection to succeed")
	}
	if g.Name != "NVIDIA GeForce GTX 1660" {
		t.Fatalf("Name = %q, want %q", g.Name, "NVIDIA GeForce GTX 1660")
	}
	if g.VRAMGB != 6.0 {
		t.Fatalf("VRAMGB = %v, want 6.0", g.VRAMGB)
	}
	if g.Kind != GPUKindNVIDIA {
		(t.Fatalf)("Kind = %q, want %q", g.Kind, GPUKindNVIDIA)
	}
}

func TestWindowsDetectorSingleObject(t *testing.T) {
	f := &fakeRunnerV2{
		output: map[string]string{
			cmdKey("powershell", "-NoProfile", "-Command", `Get-CimInstance Win32_VideoController | Select-Object Name,AdapterRAM | ConvertTo-Json`): `{"Name":"AMD Radeon RX 580","AdapterRAM":8589934592}`,
		},
	}
	g, ok := windowsDetector{}.Detect(f)
	if !ok {
		t.Fatalf("expected detection to succeed")
	}
	if g.Kind != GPUKindAMD {
		(t.Fatalf)("Kind = %q, want %q", g.Kind, GPUKindAMD)
	}
	if g.VRAMGB != 8.0 {
		t.Fatalf("VRAMGB = %v, want 8.0", g.VRAMGB)
	}
}

func TestWindowsDetectorUnknownVRAM(t *testing.T) {
	f := &fakeRunnerV2{
		output: map[string]string{
			cmdKey("powershell", "-NoProfile", "-Command", `Get-CimInstance Win32_VideoController | Select-Object Name,AdapterRAM | ConvertTo-Json`): `{"Name":"Intel UHD Graphics","AdapterRAM":0}`,
		},
	}
	g, ok := windowsDetector{}.Detect(f)
	if !ok {
		t.Fatalf("expected detection to succeed")
	}
	if g.VRAMGB != 0 {
		t.Fatalf("VRAMGB = %v, want 0", g.VRAMGB)
	}
	if g.Kind != GPUKindIntel {
		(t.Fatalf)("Kind = %q, want %q", g.Kind, GPUKindIntel)
	}
}

func TestWindowsDetectorCommandNotFound(t *testing.T) {
	f := &fakeRunnerV2{
		err: map[string]error{
			cmdKey("powershell", "-NoProfile", "-Command", `Get-CimInstance Win32_VideoController | Select-Object Name,AdapterRAM | ConvertTo-Json`): errors.New("command not found"),
		},
	}
	_, ok := windowsDetector{}.Detect(f)
	if ok {
		t.Fatalf("expected detection to fail")
	}
}

func TestWindowsDetectorMalformedJSON(t *testing.T) {
	f := &fakeRunnerV2{
		output: map[string]string{
			cmdKey("powershell", "-NoProfile", "-Command", `Get-CimInstance Win32_VideoController | Select-Object Name,AdapterRAM | ConvertTo-Json`): "not json",
		},
	}
	_, ok := windowsDetector{}.Detect(f)
	if ok {
		t.Fatalf("expected detection to fail with malformed JSON")
	}
}

func TestDetectGPUWithFakeRunner(t *testing.T) {
	f := &fakeRunnerV2{
		output: map[string]string{
			cmdKey("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits"): "NVIDIA RTX 3080, 10240",
		},
	}
	g := detectGPU(f)
	if g.Kind != "nvidia" {
		t.Fatalf("Kind = %q, want nvidia", g.Kind)
	}
	if g.Name != "NVIDIA RTX 3080" {
		t.Fatalf("Name = %q, want NVIDIA RTX 3080", g.Name)
	}
}

func TestDetectGPUNoGPU(t *testing.T) {
	f := &fakeRunnerV2{}
	g := detectGPU(f)
	if g.Kind != GPUKindNone {
		(t.Fatalf)("Kind = %q, want %q", g.Kind, GPUKindNone)
	}
}

func TestAppleChipWithFakeRunner(t *testing.T) {
	f := &fakeRunnerV2{
		output: map[string]string{
			cmdKey("sysctl", "-n", "machdep.cpu.brand_string"): "Apple M1 Pro",
		},
	}
	name := appleChip(f)
	if name != "Apple M1 Pro" {
		t.Fatalf("appleChip = %q, want Apple M1 Pro", name)
	}
}

func TestAppleChipFallback(t *testing.T) {
	f := &fakeRunnerV2{}
	name := appleChip(f)
	if name != "Apple Silicon" {
		t.Fatalf("appleChip = %q, want Apple Silicon", name)
	}
}

func TestRunWithRunner(t *testing.T) {
	f := &fakeRunnerV2{
		output: map[string]string{
			cmdKey("echo", "hello"): "hello",
		},
	}
	out, ok := runWithRunner(f, "echo", "hello")
	if !ok {
		t.Fatalf("expected runWithRunner to succeed")
	}
	if out != "hello" {
		t.Fatalf("output = %q, want hello", out)
	}
}

func TestRunWithRunnerError(t *testing.T) {
	f := &fakeRunnerV2{
		err: map[string]error{
			cmdKey("false"): errors.New("exit status 1"),
		},
	}
	_, ok := runWithRunner(f, "false")
	if ok {
		t.Fatalf("expected runWithRunner to fail")
	}
}

// recordingRunner records the context passed to Run.
type recordingRunner struct {
	ctx context.Context
}

func (r *recordingRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	r.ctx = ctx
	return "", errors.New("not found")
}

func TestDetect_PropagatesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	runner := &recordingRunner{}
	g := detectGPU(ctx, runner)
	if g.Kind != GPUKindNone {
		t.Fatalf("expected no GPU with cancelled context, got %v", g)
	}
	if runner.ctx == nil {
		t.Fatalf("expected runner.Run to be called, but it wasn't")
	}
	if runner.ctx.Err() == nil {
		t.Fatalf("expected runner.Run to receive a cancelled context")
	}
}
