package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/PabloViniegra/tui-ollama-go/internal/doctor"
)

type execDoctorRunner struct{}

func (execDoctorRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return "", fmt.Errorf("exec %s: %w", name, err)
	}
	return string(out), nil
}

func runDoctor(runner doctor.CommandRunner) int {
	checks := doctor.Run(context.Background(), runner)
	fmt.Print(doctor.Format(checks))
	if doctor.AnyMissing(checks) {
		return 1
	}
	return 0
}
