# ADR 0004: comprehensive Excel + Google Sheets function library

## Status

Proposed (2026-07-14). Extends the A1 spreadsheet model ([SPECIFICATION.md](https://github.com/uplang/tsvsheet/blob/main/SPECIFICATION.md)) with a broad, Excel-faithful function set and the value-model, call-ABI, and grammar changes that set requires. Supersedes ADR 0003 rule 10 (the 10-function set) and the worksheet-era portions of rules 1–2, 5–7, 15–19, which describe the retired two-file model.

## Goal

Implement, within reason, the non-network Excel function set plus the Google Sheets functions that do not overlap it — including array/spill and regex functions — with **Excel-faithful operator syntax**, keeping the grammar the source of truth so language bindings stay generated. Deliberately **out of scope**: network/import functions (`GOOGLEFINANCE`, `IMPORT*`, `WEBSERVICE`), the `QUERY` sublanguage, cross-sheet references (`Sheet1!A1`), cell number-format state (beyond `TEXT`), `LAMBDA`/macros, and pivot/database (`D*`) functions. These are recorded as later or never, never invented.

## Decisions

### 1. Value model — five kinds become nine

The evaluated `Value` ([internal/sheet/value.go](../../internal/sheet/value.go)) gains:

- **`kindBool`** — a distinct boolean. Comparisons now yield `TRUE`/`FALSE` (Excel-faithful), not the numbers `1`/`0` (this changes ADR 0003 rule 12). A bool coerces to `1`/`0` in arithmetic and is truthy iff `TRUE`. `TRUE()`/`FALSE()` and the literals `TRUE`/`FALSE` produce it.
- **`kindDate`** — a serial number (days since the 1899-12-30 epoch, Excel-compatible; fractional part is time-of-day) that renders ISO-8601 (`2026-07-14`) by default and coerces to its serial in arithmetic. Date functions produce and decode it. No cell number-format state is introduced; `TEXT(d, pattern)` is the formatting seam.
- **`kindArray`** — a rectangular `[][]Value` (rows × cols). Lookup and array functions consume and produce it; a top-level formula that evaluates to an array **spills** (§4).
- **Error subtypes** — add `#N/A` (lookup miss / `NA()`), `#NUM!` (domain/overflow, e.g. `SQRT(-1)`), `#NULL!` (empty range intersection), and `#SPILL!` (blocked spill, §4). The existing `#REF!` `#VALUE!` `#NAME?` `#DIV/0!` `#CIRC!` remain. Strict propagation (ADR 0003 rule 3) is unchanged **except** for the error-aware category (§2).

### 2. Call ABI — descriptor registry, three evaluation categories

`builtin func([]Value) Value` is replaced by a **descriptor** the evaluator drives:

```text
name, aliases        case-insensitive identity
minArgs, maxArgs     arity, checked centrally (variadic = maxArgs unbounded) → #VALUE! / #N/A per Excel
category             Eager | Lazy | ErrorAware
impl                 the function body
```

- **Eager** (the default, most functions): arguments are pre-evaluated. Each argument arrives as a structured `argument` — a scalar `Value`, or a **range/array** the function reads either flattened (`.cells()`, for aggregates) or 2-D (`.matrix()`, for `INDEX`/`VLOOKUP`/`TRANSPOSE`). Ranges therefore keep their shape (the current flatten-to-`[]Value` at [funcs.go:110](../../internal/sheet/funcs.go#L110) loses it and is retired). An error operand short-circuits the call to that error.
- **Lazy** (`IF`, `IFS`, `IFERROR`, `IFNA`, `AND`, `OR`, `SWITCH`, `CHOOSE`): the impl receives argument **thunks** and evaluates only what it needs — generalizing the existing special-cased `if` at [funcs.go:79](../../internal/sheet/funcs.go#L79).
- **ErrorAware** (`IFERROR`, `IFNA`, `ISERROR`, `ISERR`, `ISNA`, `ERROR.TYPE`, `AGGREGATE`): receives arguments **including** error values without short-circuit, so it can inspect them.

Functions needing the _reference itself_, not its value (`ROW`, `COLUMN`, `ROWS`, `COLUMNS`, `OFFSET`, `INDIRECT`, `ADDRESS`, `ISREF`, `CELL`), take a reference-shaped argument; the resolver exposes a cell/range reference to them without evaluating the target.

### 3. Grammar — Excel-faithful operators (grammar repo)

Changes to `TsvsheetLexer.g4`/`TsvsheetParser.g4` in [uplang/tsvsheet](https://github.com/uplang/tsvsheet), regenerated via the Docker ANTLR toolchain and re-lifted into `src/grammar`:

- **`&`** binary text concat; **`^`** power (right-assoc). Precedence, tightest first: `^`, unary `-`/`+`, `*` `/`, `+` `-`, `&`, comparisons — matching Excel.
- **`%` becomes postfix percent** (`50%` = `0.5`). Modulo moves to `MOD(a, b)`. This retires the binary `%` of ADR 0003 rule 14 (a breaking change acceptable pre-adoption).
- **`TRUE`/`FALSE`** boolean literals; **error-constant literals** (`#N/A`, `#REF!`, …) as operands.
- **Quoted strings become string literals.** In the A1 model there are no named columns, so a `"…"` token is unambiguously a string literal — the grammar/AST gains a string-literal expression node, **fixing the `"Pass"` → `#REF!` gap** (SPECIFICATION §9) and retiring ADR 0003 rules 15–16.
- `=` remains the formula marker; the SPECIFICATION §5 expression prose is updated in lockstep (grammar is the source of truth).

### 4. Spilling (dynamic arrays)

A **top-level** formula that evaluates to `kindArray` spills its cells down-and-right from the anchor into the computed grid, like modern Excel/Sheets dynamic arrays. A spill whose target cells are not empty (a literal or another formula occupies them) is `#SPILL!` at the anchor and writes nothing beyond it. Spilled cells are read-only outputs (editing one is undefined and rejected by the session). Array values used _inside_ a formula (e.g. `SUM(FILTER(...))`) never spill — only the outermost result does.

### 5. Volatility & determinism

`TODAY`, `NOW`, `RAND`, `RANDBETWEEN` are volatile in Excel. tsvsheet computes a memoized dependency graph per `Compute()`; to stay deterministic **within** a pass, volatile sources are **injected** and sampled **once per compute**: a `clock` for date/time functions and a seeded `rand` source for random functions (dependency injection per the fleet gate, so both are covered). They re-sample on the next `Compute()`. `RAND` in a stored sheet is therefore stable per render, not per read — documented as an intentional divergence from Excel's per-keystroke volatility.

### 6. Coverage as contract, at function scale

Every function is a pure `impl` with a table-driven contract test: representative results, each error/edge/arity path asserted with `errors.Is`/error-value equality. The 100% aggregate-coverage gate holds unchanged; a function is not "in" until its contract tests are. A **compatibility table** ([../capabilities/functions.md](../capabilities/functions.md)) tracks every function's status and Excel/Sheets origin, and is the single source of what is claimed.

## Consequences

- The grammar changes are cross-repo and require an ANTLR regen + re-lift; they land first (Phase 0) because the value model, ABI, and every function depend on them.
- Changing comparisons to return booleans and `%` to percent are **breaking** relative to the current formulas; acceptable pre-adoption and called out in the changelog.
- `internal/tsvt`'s retained legacy worksheet parser (ADR 0001) is pruned as part of the grammar narrowing, since the regenerated grammar drops the worksheet forms.
