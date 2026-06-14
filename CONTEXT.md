# CONTEXT

## Glossary

**Verdict** — resultado de evaluar si un modelo cabe en la RAM/VRAM del hardware detectado. Valores: `Good`, `Tight`, `No`.

**Model** — entrada del catálogo de Ollama: nombre, parámetros, cuantización, tamaño en GB.

**Result** — par `(Model, Verdict)` con el RAM necesario calculado (`NeedGB`) y el backend elegido (`cpu` / `gpu`).

**Filter** — estado de vista que restringe la lista a un Verdict específico o muestra todos (`All`, `Good`, `Tight`, `No`).

**Backend** — motor de inferencia seleccionado para un Model dado: `cpu` o `gpu`.

**Gutter** — columna de un carácter a la izquierda de cada fila que indica visualmente el Verdict de esa fila.
