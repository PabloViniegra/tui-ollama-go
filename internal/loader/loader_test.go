package loader

import (
	"context"
	"testing"
)

func TestDefault_DetectNotNil(t *testing.T) {
	s := Default()
	if s.Detect == nil {
		t.Error("Default().Detect should not be nil")
	}
}

func TestDefault_FetchNotNil(t *testing.T) {
	s := Default()
	if s.Fetch == nil {
		t.Error("Default().Fetch should not be nil")
	}
}

func TestDefault_DetectSmoke(t *testing.T) {
	s := Default()
	info := s.Detect(context.Background())
	if info.OS == "" {
		t.Error("Default().Detect: expected non-empty OS (producción debe leer runtime.GOOS)")
	}
	if info.Arch == "" {
		t.Error("Default().Detect: expected non-empty Arch")
	}
}

func TestDefault_FetchOffline(t *testing.T) {
	s := Default()
	models, err := s.Fetch(context.Background(), false, true, nil)
	if err != nil {
		t.Errorf("Fetch(offline) returned error: %v", err)
	}
	if len(models) == 0 {
		t.Error("Fetch(offline) returned empty catalog; should fall back to embedded")
	}
}
