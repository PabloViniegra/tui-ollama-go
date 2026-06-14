# ADR 0001 — TUI Visual Design System

**Status:** Accepted  
**Date:** 2026-06-14

## Context

The TUI uses hardcoded 256-color ANSI codes and has no animations. `termenv` is already in `go.mod`, which enables truecolor detection. The tool targets ML/dev users who almost always run dark terminals.

## Decisions

### 1. Truecolor palette, dark-terminal-only

Use `#RRGGBB` hex colors via lipgloss instead of ANSI integer codes. No `lipgloss.AdaptiveColor` — palette is optimized for dark terminals only.

**Alternatives considered:** Adaptive light/dark colors via `AdaptiveColor`.  
**Reason rejected:** CLI dev tool with near-100% dark terminal usage. Adaptive doubles color definitions with no real-world benefit for this audience.

### 2. Box-drawing separators, no details panel

Add `─` horizontal separators between header/list/footer regions. No side/bottom details panel for selected model.

**Alternatives considered:** Details panel showing model description and tags.  
**Reason rejected:** Scraper data doesn't include descriptions reliably. Panel would require structural refactor of `View()` and new Model state fields.

### 3. Loading spinner via Bubble Tea tick

Animate the loading state with a braille spinner (`⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`). Implemented with `tea.Tick` and a frame index in Model — no external `bubbles` dependency.

**Alternatives considered:** `charmbracelet/bubbles/spinner`.  
**Reason rejected:** Adds a dependency for ~15 lines of code. Manual impl is simpler and ponytail-correct.

### 4. Five-level typographic hierarchy

Title → Column headers → Model name → Stats → Footer/help. Bold escalates with importance; dim decreases it. Italic for footer only.
