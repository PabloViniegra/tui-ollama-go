// Package doctor audita qué herramientas del sistema están disponibles para
// `ollama-fit` (nvidia-smi, rocm-smi, ollama, etc.) e informa versión y
// presencia. Diseñado para diagnóstico rápido antes de abrir un issue o para
// smoke-tests de CI.
package doctor

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Status del check.
const (
	StatusOK      = "ok"
	StatusMissing = "missing"
)

// ToolCheck describe el resultado de auditar una herramienta.
type ToolCheck struct {
	Name    string // nombre del ejecutable, ej. "nvidia-smi"
	Args    string // args con que se invocó (para debug)
	Status  string // StatusOK o StatusMissing
	Version string // versión detectada; vacía si missing
	ErrMsg  string // mensaje de error si Status == StatusMissing
}

// CommandRunner abstrae os/exec para que los tests no necesiten herramientas
// reales instaladas.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

// defaultTimeout corta la ejecución de cada check para no colgar el doctor.
const defaultTimeout = 4 * time.Second

// Check ejecuta name con args vía runner y devuelve un ToolCheck describiendo
// el resultado. Si el runner devuelve error (o el binario no existe), Status
// es StatusMissing y ErrMsg tiene el motivo.
func Check(ctx context.Context, runner CommandRunner, name string, args ...string) ToolCheck {
	tc := ToolCheck{
		Name: name,
		Args: strings.Join(args, " "),
	}

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	out, err := runner.Run(ctx, name, args...)
	if err != nil {
		tc.Status = StatusMissing
		tc.ErrMsg = err.Error()
		return tc
	}
	tc.Status = StatusOK
	tc.Version = parseVersion(out)
	return tc
}

// parseVersion extrae una versión legible del output. Heurística: primera
// línea no vacía, palabras con dígitos (ej. "535.86.10" o "0.5.7").
func parseVersion(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if v := firstVersionToken(line); v != "" {
			return v
		}
	}
	// Si no hallamos tokens con dígitos, devolvemos la primera línea tal cual.
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

// firstVersionToken devuelve el último token de line que parece versión
// (contiene al menos un dígito). Vacío si no encuentra.
func firstVersionToken(line string) string {
	var last string
	for _, tok := range strings.Fields(line) {
		if strings.ContainsAny(tok, "0123456789") {
			last = tok
		}
	}
	return last
}

// toolSpecs es la lista de herramientas que audita Run. Editar con cuidado:
// agregar o quitar tools cambia el output y rompe expectativas del usuario.
var toolSpecs = []struct {
	name string
	args []string
}{
	{"nvidia-smi", []string{"--version"}},
	{"rocm-smi", []string{"--version"}},
	{"ollama", []string{"--version"}},
	{"sysctl", []string{"-n", "machdep.cpu.brand_string"}},
	{"uname", []string{"-mrs"}},
	{"sw_vers", []string{"-productVersion"}},
}

// Run ejecuta todos los checks predefinidos y devuelve los resultados en orden.
func Run(ctx context.Context, runner CommandRunner) []ToolCheck {
	out := make([]ToolCheck, 0, len(toolSpecs))
	for _, spec := range toolSpecs {
		out = append(out, Check(ctx, runner, spec.name, spec.args...))
	}
	return out
}

// Format devuelve una tabla legible para humanos. ErrMsg nunca aparece en
// el output (es para logs/diagnóstico).
func Format(checks []ToolCheck) string {
	if len(checks) == 0 {
		return "(no checks run)"
	}
	const nameW = 12
	const statusW = 8
	var sb strings.Builder
	fmt.Fprintf(&sb, "%-*s  %-*s  %s\n", nameW, "TOOL", statusW, "STATUS", "VERSION")
	for _, c := range checks {
		fmt.Fprintf(&sb, "%-*s  %-*s  %s\n", nameW, c.Name, statusW, c.Status, c.Version)
	}
	return sb.String()
}

// AnyMissing devuelve true si al menos un check está missing.
func AnyMissing(checks []ToolCheck) bool {
	for _, c := range checks {
		if c.Status == StatusMissing {
			return true
		}
	}
	return false
}
