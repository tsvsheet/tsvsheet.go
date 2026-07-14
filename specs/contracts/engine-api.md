# Contract: the engine API seam

Every frontend — CLI commands, the HTTP API behind `serve`, and the TUI — consumes the same engine packages. No frontend re-implements parsing, evaluation, or template editing; the browser UI is a thin client of the HTTP API, so the whole product has exactly one implementation of the language semantics.

## Packages

| Package | Responsibility | State |
| --- | --- | --- |
| `src/grammar/tsvsheet` | ANTLR-generated lexer/parser (committed, generated — DO NOT EDIT, excluded from gates) | stateless |
| `internal/tsvt` | Covered seam over the generated parser: syntax-error listener → sentinel error, parse tree → typed AST | stateless, immutable AST |
| `internal/sheet` | Value grid I/O (`.tsv` ↔ grid) and the §9 processor: header/body/final phases, reference resolution, modifiers, formula evaluation, `#REF!` propagation, per-cell trace | stateless functions over immutable inputs |
| `internal/session` | An editable worksheet: template text + data grid + computed grid, mutated by frontend edits, recomputed on change | stateful (the one sanctioned pointer-receiver type) |

`internal/` not `pkg/`: nothing is exported until an external consumer exists (visibility standard). Promotion to a public API is a later, deliberate step.

## `internal/tsvt`

- `Parse(src Source) (Template, error)` — `Source` is the raw `.tsvt` bytes. Returns the typed AST or `ErrSyntax` (an `errs.Const` carrying line/column detail via `With`). No partial trees.
- The AST mirrors the grammar 1:1: `Template` → `Line` (section marker | structural command | cell row) → `Cell` (formula | placement | literal) → `Reference` (column × row × range/matrix/grouped) and `Expr` (binary/unary/call/ref/number/string). All AST types are immutable values.

## `internal/sheet`

- `ReadTSV(r io.Reader) (Grid, error)` / `WriteTSV(w io.Writer, g Grid) error` — the grid is `[][]string`, no interpretation.
- `Compute(t tsvt.Template, g Grid) (Grid, error)` — runs §9 and returns the computed output grid. Cell-level evaluation failures are **values** (`#REF!` in the cell), never Go errors; the error return is for structural impossibilities only (e.g. a template the processor rejects).
- `Explain(t tsvt.Template, g Grid, at Address) (Trace, error)` — the diagnostic seam: which template line produced the cell, the resolved references with their values, and the evaluation result.
- `Check(t tsvt.Template) []Diagnostic` — static template diagnostics (e.g. range-scoped shift, unknown function) for the CLI `check` command.
- `Address` is a cell coordinate in spreadsheet notation (`F4`): column letters plus 1-based row. `Explain` addresses the **computed output grid**; `Session.SetDataCell` addresses the **raw data grid** (1-based data rows — header rows are presentation, not addressable, matching ADR 0003 rule 1). Parsing/formatting live beside the type (`ParseAddress`, `Address.String`).

## `internal/session`

- `New(template Source, data Grid) (*Session, error)` — parses and computes eagerly; construction fails on syntax errors.
- `Snapshot() State` — computed grid + template text + data grid + diagnostics + dirty flag, the single read model both the web UI and TUI render.
- `SetTemplate(src Source) error` — replace template text (reparse + recompute; rejected atomically on syntax error, previous state fully retained).
- `SetDataCell(a Address, v string) error` — edit raw data (growing the grid when addressing one past its bounds); recompute.
- `MarkSaved()` — clears the dirty flag after the frontend persists; every successful mutation sets it.
- `TemplateText() Source`, `DataTSV() []byte` — what gets saved. Saving itself is the frontend's job (injected writers), keeping the session filesystem-free.
- Concurrency: all `Session` methods are goroutine-safe behind an internal mutex — the HTTP handlers share one session without external locking. `Session` is the repo's one sanctioned pointer-receiver type (stateful by contract).

## HTTP API (serve) — thin projection of `Session`

| Method & path | Body → Response |
| --- | --- |
| `GET /api/state` | → `State` JSON (computed grid, template text, data grid, diagnostics) |
| `PUT /api/template` | `{"text": …}` → `State` or 422 with syntax error detail |
| `PUT /api/data/cell` | `{"row": r, "col": c, "value": …}` → `State` |
| `POST /api/save` | → writes `.tsvt` and `.tsv` back to their paths, `{"saved": true}` |
| `GET /api/explain?cell=F4` | → `Trace` JSON |

JSON keys are snake_case. The embedded web UI (`go:embed`) is a single-page grid editor speaking only this API.
