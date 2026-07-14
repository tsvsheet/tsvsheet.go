# cli — scriptable unix frontend

## Goal

Expose the engine as a strict unix-philosophy CLI: text streams in, text streams out, exit codes for scripting, no interactivity in any non-TUI command.

## Requirements

- R1: `tsvsheet render [--template PATH|-] [--data PATH|-]` — computes and writes TSV to stdout. When exactly one of the two is a file, the other defaults to stdin; `-` means stdin explicitly; template on stdin is the default piping mode (`cat sheet.tsvt | tsvsheet render --data sheet.tsv`).
- R2: `tsvsheet parse [--template PATH|-]` — emits the typed AST as JSON (snake_case) to stdout; a scripting/tooling surface, stable enough to `jq`.
- R3: `tsvsheet check [--template PATH|-]` — validates; prints diagnostics one-per-line to stderr; exit 0 clean, exit 1 diagnostics, exit 2 syntax error.
- R4: `tsvsheet explain --cell REF [--template …] [--data …]` — prints the evaluation trace for one output cell, human-readable to stdout, `--json` for machine form.
- R5: All errors go to stderr; stdout carries only the artifact; every command is pipe-safe (no prompts, no TTY assumptions); `--version` works (goreleaser ldflags).
- R6: Commands follow the template's noun/verb + `internal/app/commands` / `internal/domain` two-tier layout, urfave/cli v3, named-type Config fields, env-var sources with the `TSVSHEET_` prefix.

## Acceptance Criteria

- Piping the worked-example template with its data (`testdata/sheet-worked-example.tsvt` + `testdata/sheet-worked-example.tsv`, both committed in this repo — the grammar repo carries no `.tsv`) through `render` produces the **ADR 0003-derived** golden output (SPEC §8's shown output is documented as internally inconsistent and is not a target); the same invocation shapes are covered by tests using injected stdin/stdout.
- Exit codes asserted for: success, diagnostics, syntax error, missing input.
- 100% coverage on command and domain tiers.
