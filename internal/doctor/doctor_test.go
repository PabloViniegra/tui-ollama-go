package doctor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// fakeRunner implements CommandRunner for tests.
type fakeRunner struct {
	output map[string]string
	err    map[string]error
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	key := name + " " + strings.Join(args, " ")
	if e, ok := f.err[key]; ok {
		return "", e
	}
	if out, ok := f.output[key]; ok {
		return out, nil
	}
	return "", fmt.Errorf("command not found: %s", key)
}

func key(name string, args ...string) string {
	return name + " " + strings.Join(args, " ")
}

// -- Check ----------------------------------------------------------

func TestCheck_PresentWithVersion(t *testing.T) {
	f := &fakeRunner{
		output: map[string]string{
			key("nvidia-smi", "--version"): "NVIDIA-SMI 535.86.10",
		},
	}

	got := Check(context.Background(), f, "nvidia-smi", "--version")

	if got.Name != "nvidia-smi" {
		t.Errorf("Name = %q, want nvidia-smi", got.Name)
	}
	if got.Status != StatusOK {
		t.Errorf("Status = %q, want %q", got.Status, StatusOK)
	}
	if got.Version != "535.86.10" {
		t.Errorf("Version = %q, want 535.86.10", got.Version)
	}
	if got.ErrMsg != "" {
		t.Errorf("ErrMsg = %q, want empty", got.ErrMsg)
	}
}

func TestCheck_Missing(t *testing.T) {
	f := &fakeRunner{} // sin entry → comando no encontrado

	got := Check(context.Background(), f, "rocm-smi", "--version")

	if got.Status != StatusMissing {
		t.Errorf("Status = %q, want %q", got.Status, StatusMissing)
	}
	if got.Version != "" {
		t.Errorf("Version = %q, want empty", got.Version)
	}
	if got.ErrMsg == "" {
		t.Error("ErrMsg should be non-empty when missing")
	}
}

func TestCheck_ExplicitError(t *testing.T) {
	f := &fakeRunner{
		err: map[string]error{
			key("ollama", "--version"): errors.New("exit status 1"),
		},
	}

	got := Check(context.Background(), f, "ollama", "--version")

	if got.Status != StatusMissing {
		t.Errorf("Status = %q, want %q", got.Status, StatusMissing)
	}
	if got.ErrMsg == "" {
		t.Error("ErrMsg should be non-empty when tool returned error")
	}
}

func TestCheck_EmptyOutput(t *testing.T) {
	// Tool presente pero devuelve string vacío: status OK, version vacía.
	f := &fakeRunner{
		output: map[string]string{
			key("ollama", "--version"): "",
		},
	}

	got := Check(context.Background(), f, "ollama", "--version")

	if got.Status != StatusOK {
		t.Errorf("Status = %q, want %q", got.Status, StatusOK)
	}
	if got.Version != "" {
		t.Errorf("Version = %q, want empty", got.Version)
	}
}

// -- Run -----------------------------------------------------------

func TestRun_IncludesExpectedTools(t *testing.T) {
	f := &fakeRunner{
		output: map[string]string{
			key("nvidia-smi", "--version"): "NVIDIA-SMI 535.86.10",
			key("ollama", "--version"):     "ollama version 0.5.7",
		},
	}

	got := Run(context.Background(), f)

	// Verifica presencia de nvidia-smi y ollama en los resultados,
	// no importa el orden ni el resto del set.
	wantTools := map[string]string{ // name -> version esperada
		"nvidia-smi": "535.86.10",
		"ollama":     "0.5.7",
	}
	seen := map[string]ToolCheck{}
	for _, c := range got {
		seen[c.Name] = c
	}
	for name, wantVersion := range wantTools {
		c, ok := seen[name]
		if !ok {
			t.Errorf("Run() missing check for %q", name)
			continue
		}
		if c.Status != StatusOK {
			t.Errorf("Run()[%q].Status = %q, want %q", name, c.Status, StatusOK)
		}
		if c.Version != wantVersion {
			t.Errorf("Run()[%q].Version = %q, want %q", name, c.Version, wantVersion)
		}
	}
}

func TestRun_HandlesMissingTools(t *testing.T) {
	f := &fakeRunner{} // todas las tools → missing

	got := Run(context.Background(), f)

	if len(got) == 0 {
		t.Fatal("Run() returned empty, want at least one check")
	}
	for _, c := range got {
		if c.Status != StatusMissing {
			t.Errorf("%s.Status = %q, want %q", c.Name, c.Status, StatusMissing)
		}
		if c.ErrMsg == "" {
			t.Errorf("%s.ErrMsg should be non-empty when missing", c.Name)
		}
	}
}

// -- format --------------------------------------------------------

func TestFormat_BasicColumns(t *testing.T) {
	checks := []ToolCheck{
		{Name: "nvidia-smi", Status: StatusOK, Version: "535.86.10"},
		{Name: "rocm-smi", Status: StatusMissing, ErrMsg: "command not found"},
		{Name: "ollama", Status: StatusOK, Version: "0.5.7"},
	}
	out := Format(checks)

	for _, want := range []string{"nvidia-smi", "rocm-smi", "ollama", "535.86.10", "0.5.7"} {
		if !strings.Contains(out, want) {
			t.Errorf("Format() missing %q\n---\n%s", want, out)
		}
	}
	if strings.Contains(out, "command not found") {
		t.Errorf("Format() leaked error message into output: %q", out)
	}
}

func TestAnyMissing(t *testing.T) {
	cases := []struct {
		name string
		in   []ToolCheck
		want bool
	}{
		{"all ok", []ToolCheck{{Status: StatusOK}}, false},
		{"one missing", []ToolCheck{{Status: StatusOK}, {Status: StatusMissing}}, true},
		{"empty", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AnyMissing(tc.in); got != tc.want {
				t.Errorf("AnyMissing = %v, want %v", got, tc.want)
			}
		})
	}
}
