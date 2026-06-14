package catalog

import "testing"

func TestModels(t *testing.T) {
	got := Models()
	if len(got) == 0 {
		t.Fatal("Models() returned empty slice")
	}
	for _, m := range got {
		if m.Name == "" {
			t.Errorf("model has empty Name: %+v", m)
		}
		if m.Family == "" {
			t.Errorf("model %q has empty Family", m.Name)
		}
		if m.SizeGB <= 0 {
			t.Errorf("model %q has SizeGB <= 0: %v", m.Name, m.SizeGB)
		}
	}
}

func TestModelsContainsKnownModels(t *testing.T) {
	models := Models()
	names := make(map[string]bool, len(models))
	for _, m := range models {
		names[m.Name] = true
	}
	for _, want := range []string{"llama3.1:8b", "mistral:7b", "qwen2.5:7b"} {
		if !names[want] {
			t.Errorf("expected model %q not found in catalog", want)
		}
	}
}
