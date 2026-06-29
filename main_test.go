package main

import (
	"encoding/json"
	"testing"
)

// -- runMain integration tests --

func TestRunMainOfflineFlagParses(t *testing.T) {
	code := runMain([]string{"ollama-fit", "--offline"})
	if code != 1 {
		t.Logf("runMain(--offline) returned %d (expected 1 in non-interactive env)", code)
	}
}

func TestRunMainInvalidFlag(t *testing.T) {
	code := runMain([]string{"ollama-fit", "--invalid-flag"})
	if code != 1 {
		t.Fatalf("expected exit code 1 for invalid flag, got %d", code)
	}
}

func TestRunMain_VersionFlag(t *testing.T) {
	code := runMain([]string{"ollama-fit", "--version"})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRunMain_ShortVersionFlag(t *testing.T) {
	code := runMain([]string{"ollama-fit", "-V"})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRunMain_VersionFlagBeforeSubcommand(t *testing.T) {
	for _, args := range [][]string{
		{"ollama-fit", "--version"},
		{"ollama-fit", "-V"},
	} {
		if code := runMain(args); code != 0 {
			t.Errorf("runMain(%v) = %d, want 0", args, code)
		}
	}
}

func TestRunMain_DoctorDispatch(t *testing.T) {
	code := runMain([]string{"ollama-fit", "doctor"})
	if code != 0 && code != 1 {
		t.Errorf("runMain(doctor) = %d, want 0 o 1 (subcomando doctor corrió)", code)
	}
}

func TestRunMain_LocalDispatch(t *testing.T) {
	code := runMain([]string{"ollama-fit", "local"})
	if code != 0 && code != 3 {
		t.Errorf("runMain(local) = %d, want 0 (OK) o 3 (ollama ausente)", code)
	}
}

// -- schema tests (fitOutputSchema embedded in main.go) --

func TestFitOutputSchema_ValidJSON(t *testing.T) {
	if !json.Valid(fitOutputSchema) {
		t.Errorf("fitOutputSchema is not valid JSON:\n%s", fitOutputSchema)
	}
}

func TestFitOutputSchema_HasRequiredFields(t *testing.T) {
	var raw map[string]any
	if err := json.Unmarshal(fitOutputSchema, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	required, ok := raw["required"].([]any)
	if !ok {
		t.Fatal("schema has no 'required' array")
	}
	want := map[string]bool{
		"verdict":      false,
		"backend":      false,
		"need_gb":      false,
		"available_gb": false,
		"reason":       false,
		"model":        false,
	}
	for _, r := range required {
		if s, ok := r.(string); ok {
			if _, exists := want[s]; exists {
				want[s] = true
			}
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("schema missing required field %q", name)
		}
	}
}
