# ADR 0005 — Rich interactive model: reference visibility, structural edits, embedded sub-sheets

## Status

Accepted (2026-07-14).

## Context

The web and TUI frontends were a faithful but minimal projection of the engine: select a cell, edit its source, recompute. To make the interactive experience substantially richer we want three capabilities, in ascending order of ambition:

1. **Reference visibility** — when a cell is selected, show which cells its formula reads (precedents) and which cells read it (dependents). Spreadsheets rarely make the dependency graph visible; doing so is a genuine usability win and pure engine information.
2. **Structural edits** — insert and delete whole rows and columns, with A1 references in every formula rewritten to follow the move (shift on insert/delete; a reference to a deleted cell becomes `#REF!`), exactly like a conventional spreadsheet.
3. **Embedded sub-sheets** — a cell whose value is the computed output of an _entire other sheet_: a spreadsheet used as a function. This is the headline feature and does not exist in conventional tools.

The binding constraint is the project's **grammar-first** rule: the formula language is defined by the ANTLR grammar in `uplang/tsvsheet`, and anything the grammar can express must be expressed there, not bolted on as host-side wrapper logic.

## Decision

### Reference visibility — pure engine query, no language change

Add two total functions to the engine over the existing AST:

- `Sheet.Precedents(at) []Span` — the cell/range references a formula reads, as resolved 0-based `Span`s (a `Span` is a `From`/`To` `Address` pair; a single cell has `From == To`). Built by walking the parsed formula's reference operands (`walkRefs`) — the same walk `Explain` already uses.
- `Sheet.Dependents(at) []Address` — the reverse edge: every formula cell whose precedents cover `at`.

Both are value methods returning value types; no grammar or spec change — this is information the parse tree already holds.

### Structural edits — AST reference rewriting, still grammar-native

`Sheet.InsertRow/DeleteRow/InsertCol/DeleteCol` return a new `Sheet` (the engine stays immutable). The grid is reshaped, and **every formula is re-serialized with its references shifted** by transforming the parsed AST (`CellRef.Row`/`Col`) and rendering it back through `RenderExpr` — never by string-munging source. A reference whose target is deleted renders as the `#REF!` error literal, which the existing evaluator already propagates. Because the transform is AST-in/AST-out and re-serialized through the existing renderer, the grammar remains the single source of truth for formula syntax; the shift is a semantic transformation layered over the parse tree (SPECIFICATION §7 territory).

### Embedded sub-sheets — three grammar-native builtins

A sub-sheet is referenced by file path and used as a function. Three builtins, **all ordinary function calls the existing grammar already admits** (`functionCall : (NAME|COL) NUMBER? LPAREN argList? RPAREN`), so there is **no grammar change** — only new semantics in the engine and new prose in `SPECIFICATION.md`:

- `OUTPUT(expr)` — marks the cell it occupies as the sheet's single output cell; its value is `expr` (identity). A sheet with an `OUTPUT` cell can be embedded. Two `OUTPUT` cells, or embedding a sheet with none, is `#REF!`.
- `SHEET(path, args…)` — loads the `.tsvt` at `path`, computes it, and returns its `OUTPUT` cell's value. The embedding cell's value _is_ the sub-sheet's output.
- `INPUT(n)` — inside a sub-sheet, resolves to the nth argument passed by the embedding `SHEET(…)` call (1-based), making the sub-sheet a parameterized function. Out of range, or evaluated in a sheet that was not embedded, is `#REF!`.

Safety and termination:

- **Path containment** — `path` is resolved relative to the embedding sheet's own file and must stay within the root sheet's base directory; a `..` escape or absolute path outside the base is `#REF!`. The engine is handed a resolver (dependency injection) so it stays filesystem-free and testable, and the base directory is fixed by the frontend.
- **Cross-sheet cycle detection** — the embedding chain is tracked; a sheet that transitively embeds itself is `#CIRC!`, mirroring intra-sheet cycle handling.

The UI renders an embedding cell as a nested mini-grid preview of the sub-sheet, so "an entire sheet inside a cell" is literal on screen while the computed value remains the cell's scalar.

## Consequences

- The engine gains a small, well-typed query surface (`Span`, `Precedents`, `Dependents`) and structural constructors, all value-typed and 100%-covered.
- `SHEET`/`OUTPUT`/`INPUT` are the first builtins with side inputs (a file resolver, an argument stack). They are threaded through the compute pass via injected collaborators, not globals, preserving determinism and testability.
- No grammar regeneration is required — the headline feature rides entirely on existing syntax, which is the strongest possible evidence that the grammar-first design was right.
- The `.tsvt` format now has cross-file semantics; `SPECIFICATION.md` gains an "Embedded sheets" section and the containment/cycle rules are recorded there, with anything underspecified marked `[open]` rather than invented.
