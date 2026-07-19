# tsvsheet.go

> **A spreadsheet for plain text.** The Go implementation of [tsvsheet](https://github.com/tsvsheet/tsvsheet): a `.tsvt` **is** the spreadsheet — a single TAB-separated grid whose cells are literal values or `=formulas` that address other cells in A1 notation (`B2`, `D2:D5`), computed in place — editable from the CLI, the browser, or the terminal, all through one engine.

This is the CLI/library implementation repo (module `github.com/tsvsheet/tsvsheet.go`, binary `cmd/tsv/`). It lifts the grammar repo's ANTLR-generated Go parser and reuses only its **expression sublanguage** (via `tsvt.ParseFormula`) to compile each cell's formula; the surrounding **A1 single-file spreadsheet model is layered here**.

> **Model pivot (done).** The repo was reoriented from the salvaged 2011 two-file worksheet (a `.tsvt` template of `=header`/`=body`/`=final` sections over a companion `.tsv` grid) to the single-file A1 spreadsheet above, and the fold-back is complete: the grammar has been narrowed to the A1 expression language with Excel-faithful operators (`^`, `&`, postfix `%`, `TRUE`/`FALSE` and error-value literals) — the legacy worksheet/section/modifier rules and their `tsvt` parser are pruned — and `SPECIFICATION.md` (grammar repo) describes this model. Quoted strings are string literals (the former `#REF!` gap is closed). The comprehensive Excel + Google Sheets **function library** is being built out over this foundation per [ADR 0004](https://github.com/tsvsheet/.projects/blob/main/specs/tsvsheet/decisions/0004-excel-sheets-functions.md) and [specs/capabilities/functions.md](https://github.com/tsvsheet/.projects/blob/main/specs/tsvsheet/capabilities/functions.md), in phases behind the green gate. The older [specs/](https://github.com/tsvsheet/.projects/tree/main/specs/tsvsheet/) capability docs and ADRs 0001–0003 still describe the two-file worksheet and are stale pending their own refresh.

## Architecture — one engine, three frontends

Every frontend consumes the same engine; none re-implements parsing or evaluation ([ADR 0002](https://github.com/tsvsheet/.projects/blob/main/specs/tsvsheet/decisions/0002-one-engine-thin-clients.md)).

- `src/grammar/tsvsheet` — the committed ANTLR-generated parser (package `tsvsheetgrammar`, `// Code generated … DO NOT EDIT`). Regenerate with `make go` in the grammar repo and copy `gen/go/*.go` here; excluded from the gates via [Makefile.local](../Makefile.local).
- `internal/tsvt` — the covered seam over the generated parser: source → immutable typed AST, or `constants.ErrSyntax`. No ANTLR type escapes this package.
- `internal/sheet` — the A1 spreadsheet engine: grid I/O, A1 reference resolution, the §11 expression evaluator with error values (`#REF!`/`#DIV/0!`/`#CIRC!`), dependency-ordered memoized `Compute`, immutable `Set`, `Check` diagnostics, `Explain` traces.
- `internal/session` — the one mutable editing model (the repo's sole pointer-receiver type), backing serve and tui.
- `internal/cli` — urfave/cli v3 commands: `render` `parse` `check` `explain` `serve` `tui`, unix stdin/stdout discipline.
- `internal/serve` — HTTP JSON API + embedded browser spreadsheet (`go:embed`, thin client of the session).
- `internal/tui` — bubbletea terminal editor over the same session.

## Non-negotiables

- **The engine is the single source of truth for semantics.** A new capability the web or TUI needs is added to `internal/session`/`internal/sheet` and consumed by both — never duplicated in a frontend. The ANTLR JavaScript target is deliberately unused (ADR 0002).
- **`[open]` SPECIFICATION items are decisions, not inventions.** Anything the 2011 source underspecified is pinned in [ADR 0003](https://github.com/tsvsheet/.projects/blob/main/specs/tsvsheet/decisions/0003-open-semantics.md) with a rationale; the §8 worked example is documented as internally inconsistent and is **not** a conformance target. New semantic choices extend that ADR.
- **The full gomatic Go gate applies:** `make check` green — gofumpt, vet, staticcheck, golangci (gocognit ≤ 7), govulncheck, **100% aggregate coverage** (every package, including `cmd/`; `src/grammar` excluded). Errors are `errs.Const` sentinels in `internal/constants`; no `fmt.Errorf`/`errors.New`. Value receivers except `session.Session`.
