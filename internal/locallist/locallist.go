// Package locallist cruza lo reportado por `ollama list` con el catálogo
// embebido para mostrar los modelos ya instalados y cómo le va cada uno en
// tu hardware. Diseñado para diagnóstico rápido sin abrir la TUI.
package locallist

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/PabloViniegra/tui-ollama-go/internal/catalog"
	"github.com/PabloViniegra/tui-ollama-go/internal/eval"
	"github.com/PabloViniegra/tui-ollama-go/internal/hardware"
)

// LocalEntry es una fila parseada de `ollama list`.
type LocalEntry struct {
	Name string // nombre del modelo, p.ej. "qwen2.5:7b"
	Size string // tamaño reportado por ollama (ej. "4.7 GB", o "-" para cloud)
}

// Model agrupa un modelo local con su evaluación contra el hardware detectado.
type Model struct {
	Name      string      // nombre del modelo local
	LocalSize string      // tamaño tal cual lo reporta ollama list
	Result    eval.Result // evaluación contra el hw del usuario
}

// CommandRunner abstrae os/exec (mismo patrón que internal/hardware e internal/doctor).
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

// ParseOllamaList extrae las entradas (Name, Size) del output tabulado de
// `ollama list`. Las columnas del formato real están alineadas con 2+
// espacios/tabs entre sí, así que split por `\s{2,}` mantiene tokens como
// "4.7 GB" juntos. Líneas con menos de 3 columnas se descartan. El header
// (NAME como primera columna) se ignora.
func ParseOllamaList(output string) []LocalEntry {
	var out []LocalEntry
	for _, line := range strings.Split(output, "\n") {
		cols := columnSplit.Split(strings.TrimSpace(line), -1)
		if len(cols) < 3 {
			continue
		}
		if cols[0] == "NAME" {
			continue
		}
		out = append(out, LocalEntry{Name: cols[0], Size: cols[2]})
	}
	return out
}

// columnSplit separa por 2+ whitespace chars (alineación de columnas ASCII).
var columnSplit = regexp.MustCompile(`\s{2,}`)

// EvaluateLocal corre `ollama list`, parsea, y evalúa cada modelo contra
// el hardware del usuario usando el catálogo provisto. Modelos cloud (size
// == "-") y modelos no presentes en el catálogo se filtran.
// Devuelve error si el runner falla; en ese caso []Model es nil.
func EvaluateLocal(ctx context.Context, runner CommandRunner, hw hardware.Info, models []catalog.Model) ([]Model, error) {
	if runner == nil {
		return nil, errors.New("locallist: nil runner")
	}
	out, err := runner.Run(ctx, "ollama", "list")
	if err != nil {
		return nil, fmt.Errorf("ollama list: %w", err)
	}
	entries := ParseOllamaList(out)
	results := make([]Model, 0, len(entries))
	for _, e := range entries {
		if e.Size == "-" {
			continue // cloud variant; sin tamaño no se evalúa
		}
		m, ok := findInCatalog(models, e.Name)
		if !ok {
			continue // no en catálogo; usuario puede correr --refresh
		}
		results = append(results, Model{
			Name:      e.Name,
			LocalSize: e.Size,
			Result:    eval.Evaluate(hw, m),
		})
	}
	return results, nil
}

// findInCatalogo busca un modelo por nombre case-insensitive exacto.
// Es interno: si crece, se exporta. Misma semántica que findModel en main.go,
// pero aislado para no acoplar paquetes.
func findInCatalog(models []catalog.Model, name string) (catalog.Model, bool) {
	for _, m := range models {
		if strings.EqualFold(m.Name, name) {
			return m, true
		}
	}
	return catalog.Model{}, false
}

// Format devuelve una tabla compacta con los resultados. Si no hay
// resultados, devuelve un mensaje informativo.
func Format(results []Model) string {
	if len(results) == 0 {
		return "no hay modelos locales con tamaño evaluable (corré `ollama pull <modelo>` y/o `ollama-fit --refresh`)\n"
	}
	const nameW, sizeW, verdictW, needW, backendW = 22, 8, 8, 8, 10
	var sb strings.Builder
	fmt.Fprintf(&sb, "%-*s  %-*s  %-*s  %-*s  %-*s\n",
		nameW, "NAME", sizeW, "SIZE", verdictW, "VERDICT", needW, "NEED", backendW, "BACKEND")
	for _, r := range results {
		fmt.Fprintf(&sb, "%-*s  %-*s  %-*s  %-*s  %-*s\n",
			nameW, r.Name,
			sizeW, r.LocalSize,
			verdictW, r.Result.Verdict.String(),
			needW, fmt.Sprintf("%.1f GB", r.Result.NeedGB),
			backendW, r.Result.Backend)
	}
	return sb.String()
}
