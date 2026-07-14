# session — the stateful editing hub

## Goal

One mutable worksheet model (`internal/session`) that every interactive frontend (serve, tui) drives, so editing capabilities exist exactly once.

## Requirements

- R1: `New(template Source, data Grid) (*Session, error)` parses and computes eagerly; a syntax error fails construction with `ErrSyntax`.
- R2: `SetTemplate(src)` is atomic: on syntax error the session keeps the previous template, data, computed grid, and dirty state unchanged, and returns the error for the frontend to display.
- R3: `SetDataCell(a Address, v string)` edits the raw data grid (growing it if the address is one past the current bounds) and recomputes; every successful mutation recomputes synchronously before returning.
- R4: `Snapshot() State` returns the complete read model — computed grid, template text, raw data grid, diagnostics, and dirty flag — as a value; frontends never reach into session internals.
- R5: Dirty tracking: any successful mutation sets dirty; `MarkSaved()` clears it after the frontend persists; `Snapshot().Dirty` is how the TUI warns on quit and the web UI enables Save.
- R6: `Session` methods are safe for concurrent use (internal mutex); it is the repo's one sanctioned pointer-receiver type, justified at the type.
- R7: The session never touches the filesystem; frontends persist `TemplateText()`/`DataTSV()` through injected writers.

## Acceptance Criteria

- Tests cover: eager compute on New, atomic rejection (state identical after a failed SetTemplate), grow-on-append edits, dirty lifecycle (new → clean, edit → dirty, MarkSaved → clean), concurrent mutation under `-race`, and every error path via `errors.Is`.
- 100% statement coverage.
