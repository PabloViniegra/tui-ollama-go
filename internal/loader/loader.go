// Package loader encapsula las dos dependencias externas que el CLI necesita:
// detectar hardware y obtener el catálogo. Permite inyectar fixtures en tests
// y otros contextos (p.ej. un servidor web de métricas) sin acoplarse a la
// red ni al sistema.
package loader

import (
	"context"

	"github.com/PabloViniegra/tui-ollama-go/internal/catalog"
	"github.com/PabloViniegra/tui-ollama-go/internal/hardware"
)

// Source agrupa los dos puntos de inyección que el CLI usa: detección de
// hardware y fetch del catálogo. Las funciones se llaman igual que las del
// paquete original; basta con sustituirlas en tests.
type Source struct {
	Detect func(ctx context.Context) hardware.Info
	Fetch  func(ctx context.Context, refresh, offline bool, progress func(string)) ([]catalog.Model, error)
}

// Default devuelve un Source cableado a las implementaciones de producción.
// Producción usa esto; los tests construyen un Source custom.
func Default() *Source {
	return &Source{
		Detect: hardware.Detect,
		Fetch:  catalog.Fetch,
	}
}
