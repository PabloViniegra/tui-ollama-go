// Package tui implementa la interfaz de terminal con Bubble Tea.
package tui

// UI strings en español centralizados.
const (
	msgStatus       = "ESTADO"
	msgModel        = "MODELO"
	msgParams       = "PARÁM"
	msgQuant        = "CUANT"
	msgMemory       = "MEMORIA"
	msgBackend      = "BACKEND"
	msgNoResults    = "  (sin resultados para el filtro/búsqueda actual)"
	msgGood         = "Va bien"
	msgTight        = "Justo"
	msgNoFit        = "No cabe"
	msgFilterAll    = "todos"
	msgFilterGood   = "va bien"
	msgFilterTight  = "justo"
	msgFilterNo     = "no cabe"
	msgSearchPrompt = " buscar: %s_   (enter aplica · esc limpia)"
	msgFooterHelp   = "↑/↓ mover · pgup/pgdn · g/G inicio/fin · f filtro [%s] · / buscar · q salir"
	msgCPUHeader    = "CPU  %s · %d núcleos    RAM  %.1f GB"
	msgGPUHeader    = "GPU  %s"
)
