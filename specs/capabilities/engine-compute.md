# engine-compute — the §9 processor

## Goal

Compute an output grid from a `.tsv` value grid plus a parsed template, implementing SPEC §9 with the [ADR 0003](../decisions/0003-open-semantics.md) semantics, and expose per-cell traces for diagnostics.

## Requirements

- R1: `ReadTSV`/`WriteTSV` round-trip a grid losslessly (tabs and newlines structural, everything else verbatim; final newline emitted).
- R2: Phases: `=header(n)` binds column names from the next `n` lines; `=body` lines apply per data row with row-relative resolution; `=final` lines apply once after all rows; a template with no section markers is all body (§4).
- R3: Reference resolution implements §5 fully (letters, 0-based `[i,j]` numerics with negatives from the end, named columns, `$` last, `*` wildcards, ranges, matrices, grouped ranges) under the ADR 0003 rules.
- R4: Formula evaluation implements §11 precedence, the ADR 0003 function set, error values (`#REF!`, `#VALUE!`, `#NAME?`, `#DIV/0!`) and strict-with-lazy-`if` propagation.
- R5: Structural modifiers `>` `<` `!` work for single column/cell/per-row selections with the ADR 0003 rule-13 split (standalone command = insert-before/insert-after/delete; cell position = shift/swap/delete per §6); range-scoped forms are rejected with a diagnostic (ADR 0003 rule 7).
- R6: `Explain(t, g, at)` returns the producing template line, each referenced address with its resolved value, and the result — the data behind `tsvsheet explain`.
- R7: `Check(t)` returns static diagnostics (range-scoped modifiers, unknown functions, header-name references without a header section) without needing data.
- R8: Evaluation is a single top-to-bottom pass; no recalculation fixpoint (§9).

## Acceptance Criteria

- Golden tests: a corpus of template+data pairs (including the §8 template) with committed expected TSV outputs derived from ADR 0003; the §8 discrepancies are asserted as _our_ documented values.
- Every ADR 0003 rule has at least one test exercising it, including each error value and each rejection path.
- 100% statement coverage on `internal/sheet`, failure paths asserted with `errors.Is`.

## Non-Functional

- A 10k-row × 26-column sheet computes in under a second (no quadratic reference resolution).
