# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
go mod tidy          # download deps, generate go.sum (required on first clone)
go run .             # scrapes ollama.com on first run (~seconds); caches for 24h
go build -o ollama-fit .

# Flags
go run . --refresh   # bypass cache, re-scrape ollama.com
go run . --offline   # skip network, use embedded fallback catalog
```

Test suite exists across all packages (hardware, catalog, eval, tui, main). Run `go test ./...`.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.

## Architecture

**Pattern:** Modular CLI Monolith — `cmd/` package extraction  
**Decision date:** 2026-06-29  
**Status:** Accepted  
**ADR:** /docs/adr/0002-cmd-package-extraction.md

### Guiding principles

- `main.go` is an entry point only — no logic, no types, no formatters
- One file per subcommand in `internal/cmd/`
- `loader.Source` is the only infra abstraction needed — do not add more ports without a concrete second adapter
- Inner packages (`catalog`, `hardware`, `eval`, `tui`, `loader`, `doctor`, `locallist`) never import `internal/cmd`

### Folder structure

```
main.go                 # os.Exit(cmd.Run(os.Args))
internal/
  cmd/
    root.go             # Run() dispatcher + TUI default flow
    fit.go              # fit subcommand + output formatting
    doctor.go           # doctor subcommand + execDoctorRunner adapter
    local.go            # local subcommand + execLocalRunner adapter
    version.go          # printVersion
  catalog/
  hardware/
  eval/
  tui/
  loader/               # loader.Source: hardware.Detect + catalog.Fetch ports
  doctor/
  locallist/
```

### Rules for contributors

- `main` imports `internal/cmd` only
- `internal/cmd` may import any `internal/*` package
- No `internal/*` package imports `internal/cmd` — compile error if violated
- New subcommand = one new file in `internal/cmd/` + one case in `root.go`
- Formatters and adapter structs for a subcommand live in that subcommand's file, not in a separate `format/` or `adapters/` package

### Why we chose this

`main.go` had grown to 330 lines mixing routing, adapters, formatters, and orchestration. Extracting to `internal/cmd/` restores `main.go` as a pure entry point with zero logic, matching Go idioms for CLI tools and making each subcommand independently testable.

## Agent skills

### Issue tracker

Issues live in GitHub Issues (`PabloViniegra/tui-ollama-go`). See `docs/agents/issue-tracker.md`.

### Triage labels

Default canonical label strings (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context — `CONTEXT.md` + `docs/adr/` at repo root. See `docs/agents/domain.md`.
