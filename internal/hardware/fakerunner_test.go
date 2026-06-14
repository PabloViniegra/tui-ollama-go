package hardware

import (
	"context"
	"errors"
	"testing"
)

// fakeRunner implements CommandRunner for tests.
type fakeRunner struct {
	output map[string]string
	err    map[string]error
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	key := name
	if len(args) > 0 {
		key += " " + args[0]
	}
	if e, ok := f.err[key]; ok {
		return "", e
	}
	if out, ok := f.output[key]; ok {
		return out, nil
	}
	return "", errors.New("command not found: " + key)
}

func TestFakeRunnerHappyPath(t *testing.T) {
	f := &fakeRunner{
		output: map[string]string{
			"nvidia-smi --query-gpu=name,memory.total": "NVIDIA GeForce RTX 4070, 12288",
		},
	}
	out, err := f.Run(context.Background(), "nvidia-smi", "--query-gpu=name,memory.total")
	if err != nil {
		(t.Fatalf)("unexpected error: %v", err)
	}
	if out != "NVIDIA GeForce RTX 4070, 12288" {
		(t.Fatalf)("unexpected output: %q", out)
	}
}

func TestFakeRunnerErrorInjection(t *testing.T) {
	f := &fakeRunner{
		err: map[string]error{
			"nvidia-smi --query-gpu=name,memory.total": errors.New("command not found"),
		},
	}
	_, err := f.Run(context.Background(), "nvidia-smi", "--query-gpu=name,memory.total")
	if err == nil {
		(t.Fatalf)("expected error, got nil")
	}
}

func TestExecRunnerImplementsCommandRunner(t *testing.T) {
	// Verify execRunner satisfies CommandRunner at compile time.
	var _ CommandRunner = execRunner{}
}

func TestDetectorChainHasNVIDIA(t *testing.T) {
	found := false
	for _, d := range detectorChain {
		if d.name == "nvidia" {
			found = true
			break
		}
	}
	if !found {
		(t.Fatalf)("detectorChain missing nvidia entry")
	}
}

func TestNvidiaDetectorWithFakeRunner(t *testing.T) {
	f := &fakeRunner{
		output: map[string]string{
			"nvidia-smi --query-gpu=name,memory.total": "NVIDIA GeForce RTX 4070, 12288",
		},
	}
	g, ok := nvidiaDetector{}.Detect(context.Background(), f)
	if !ok {
		(t.Fatalf)("expected detection to succeed")
	}
	if g.Name != "NVIDIA GeForce RTX 4070" {
		(t.Fatalf)("Name = %q, want %q", g.Name, "NVIDIA GeForce RTX 4070")
	}
	if g.VRAMGB != 12.0 {
		(t.Fatalf)("VRAMGB = %v, want 12.0", g.VRAMGB)
	}
	if g.Kind != "nvidia" {
		(t.Fatalf)("Kind = %q, want %q", g.Kind, "nvidia")
	}
}

func TestNvidiaDetectorErrorPropagation(t *testing.T) {
	f := &fakeRunner{
		err: map[string]error{
			"nvidia-smi --query-gpu=name,memory.total": errors.New("command not found"),
		},
	}
	_, ok := nvidiaDetector{}.Detect(context.Background(), f)
	if ok {
		(t.Fatalf)("expected detection to fail")
	}
}

func TestDetectDoesNotPanic(t *testing.T) {
	// Smoke test: Detect() must not panic on any GOOS.
	_ = Detect(context.Background())
}
