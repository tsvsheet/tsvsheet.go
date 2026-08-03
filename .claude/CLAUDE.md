# tsvsheet.go

> **A spreadsheet in plain text.** The Go CLI and frontends for [tsvsheet](https://github.com/tsvsheet/tsvsheet): a `.tsvt` **is** the spreadsheet — a single TAB-separated grid whose cells are literal values or `=formulas` that address other cells in A1 notation (`B2`, `D2:D5`), computed in place — editable from the CLI, the browser, or the terminal, all through one engine.

This repo is the **consumer**, not the engine: module `github.com/tsvsheet/tsvsheet.go`, binary `cmd/tsv/`. Parsing, evaluation, the function library, the document layer, the edit language and every budget live in [go-tsvsheet](https://github.com/tsvsheet/go-tsvsheet) and arrive here as a pinned dependency ([rules.md R1](https://github.com/tsvsheet/.projects/blob/main/specs/tsvsheet/rules.md)). Nothing in this repo may re-implement a semantic — if the web or TUI needs a capability, it is added to the engine and consumed by both.

## Architecture — one engine, three frontends

- `internal/cli` — the urfave/cli v3 command tree and the only place flags are bound: `render` `parse` `from-json` `check` `explain` `eval` `apply` `serve sheet|api` `tui` `data` `man` `completion`. Command logic lives in stream-injected functions (`Streams{In,Out,Err}`) so it is testable without a terminal; the `cli.Command` wrappers bind flags and streams only. Unix stdin/stdout discipline throughout (`-` is stdin).
- `internal/session` — the one mutable editing model and the repo's sole pointer-receiver type; backs both `serve sheet` and `tui`.
- `internal/serve` — the HTTP JSON API plus the embedded browser spreadsheet (`go:embed`), a thin client of the session. `serve api` instead composes [tsvsheet.api](https://github.com/tsvsheet/tsvsheet.api)'s handler and confined store — that server is never reimplemented here.
- `internal/tui` — the bubbletea terminal editor over the session, **plus the view-only `Pager`** for documents past the resident budget: a deliberately separate, smaller model that reads viewport windows through the engine's `WindowedSheet` while the file stays on disk.
- `internal/loader` — the filesystem `tsvsheet.Loader` for `SHEET(…)` and `"file"!A1` references: confined to a root via `os.Root` by default, unconfined behind an explicit flag. Every sibling load is budget-bounded like the top document.
- `internal/importer` — the real `net/http` Fetcher for the `IMPORT*` family: operator-supplied host allowlist, https-only, bounded, with a cross-pass cache and explicit refresh ([R7](https://github.com/tsvsheet/.projects/blob/main/specs/tsvsheet/rules.md)).
- `internal/dataserve` — `tsv data`, publishing a directory as the base that relative import sources resolve against.
- `internal/refresh` — the auto-refresh cadence shared by serve and tui: an explicit interval, else the union of the sheet's own `volatile(…)` schedules (Go durations or isnow patterns), soonest wins.
- `internal/constants` — this repo's `errs.Const` sentinels. No `fmt.Errorf`, no `errors.New`.

## Any size, by policy

A document loads through one bounded path ([R16](https://github.com/tsvsheet/.projects/blob/main/specs/tsvsheet/rules.md)): a census scan decides, in O(index) memory with no cell parsed, whether it fits `Limits.ResidentCells`. In budget, everything behaves exactly as it always did. Over budget, **every** load path refuses — naming the census, the budget, and the two remedies — except `tsv tui`, which pages it view-only. `--max-cells` raises the ceiling and makes any size editable; a budget says what this run will pay for, never what the language can do. When touching a load path, keep the refusal O(index) (census *before* buffering) and route it through `vetCensus` so there is one message, not five.

## Non-negotiables

- **The engine is the single source of truth for semantics.** New behaviour goes to go-tsvsheet and is consumed here; a frontend never parses or evaluates on its own.
- **The full gomatic Go gate applies:** `make check` green — gofumpt, vet, staticcheck, golangci (gocognit ≤ 7), govulncheck, **100% aggregate coverage** (every package, including `cmd/`). Value receivers except `session.Session`. Run it in the pinned image, which is stricter than a local `${GOBIN}`: `docker run --rm --platform linux/amd64 -v "$PWD":/work -w /work ghcr.io/nicerobot/tools.build/ci/go:v2 make ci`.
- **Green is the floor, not the finish** ([R13](https://github.com/tsvsheet/.projects/blob/main/specs/tsvsheet/rules.md)): a non-trivial change gets an adversarial pass before it reaches this public repo, and lands only when every finding is fixed and re-verified.
- **Integration tests are build-tag gated** (`//go:build integration`, `make test-tag TAG=integration`) so the fleet's 1:1 test-file convention holds. They drive the real command tree and both HTTP servers; no CI job runs them yet (see the org [friction log](https://github.com/tsvsheet/.projects/blob/main/friction-log.md)).
