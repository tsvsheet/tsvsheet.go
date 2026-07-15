# functions — the Excel + Google Sheets formula library

## Goal

A comprehensive, Excel-faithful function library for `.tsvt` formulas — the non-network Excel set plus non-overlapping Google Sheets functions, arrays/spill and regex included — implemented as a descriptor registry over the ANTLR expression AST, so the grammar stays the source of truth and language bindings stay generated. Design decisions (value model, call ABI, grammar changes, spilling, volatility) are pinned in [ADR 0004](../decisions/0004-excel-sheets-functions.md).

## Requirements

- **R1 — value model.** `Value` supports number, text, boolean, date (serial, ISO default render), array (2-D), empty, and the error values `#REF! #VALUE! #NAME? #DIV/0! #CIRC! #N/A #NUM! #NULL! #SPILL!`, each with defined coercion and propagation (ADR 0004 §1).
- **R2 — call ABI.** Functions are registered as descriptors with case-insensitive name/aliases, central arity checking, an evaluation category (Eager / Lazy / ErrorAware), and structured arguments (scalar / range-or-array read flat or 2-D / reference) (ADR 0004 §2). Adding a function is one descriptor plus a pure `impl`.
- **R3 — grammar.** Excel-faithful operators (`&` concat, `^` power, postfix `%` percent, `TRUE`/`FALSE` and error-constant literals, string literals) are in the grammar; the SPECIFICATION §5 prose is updated in lockstep; `MOD()` replaces binary `%` (ADR 0004 §3).
- **R4 — spilling.** A top-level array result spills down-and-right from its anchor; a blocked spill is `#SPILL!`; inner array results do not spill (ADR 0004 §4).
- **R5 — determinism.** Volatile functions (`TODAY`/`NOW`/`RAND`/`RANDBETWEEN`) sample injected clock/rand sources once per `Compute()` (ADR 0004 §5).
- **R6 — coverage as contract.** Every function has table-driven contract tests (results + every error/edge/arity path) and is absent from the compatibility table until they pass. The 100% aggregate-coverage gate holds.
- **R7 — diagnostics.** `Check` reports unknown functions, wrong-arity calls it can prove statically, and non-A1 references, naming the offending cell.

## Acceptance Criteria

- Every function listed "done" below computes its representative Excel/Sheets result and returns the correct error value for each documented failure, asserted in tests.
- `make check` is green (clean cache) after every phase: gofumpt, vet (grammar-excluded), staticcheck, golangci (gocognit ≤ 7), govulncheck, 100.0% aggregate coverage; `errs.Const` sentinels only; value receivers except `session.Session`.
- The compatibility table below matches the registry exactly — no function is claimed that isn't registered and tested, and none is registered that isn't listed.
- Grammar changes round-trip: the `.g4` regenerates, the parser re-lifts to `src/grammar`, and existing formulas that remain valid still parse.

## Phases

Each phase lands behind a green gate; the compatibility table's Phase column is the plan of record.

| Phase | Scope |
| --- | --- |
| **0 — foundation** | Value-model extension (R1), call-ABI descriptor registry (R2), grammar changes + regen + re-lift (R3), spill machinery (R4), volatile-source injection (R5). No new user functions beyond what the ABI needs; the existing 9 are ported to descriptors. Prune the legacy worksheet parser (ADR 0004 consequence). |
| **1 — math & trig** | Rounding family, powers/roots/logs, trig, combinatorics, `SUMIF(S)`/`SUMPRODUCT`. |
| **2 — logical & information** | `IF`/`IFS`/`IFERROR`/`IFNA`/`AND`/`OR`/`NOT`/`XOR`/`SWITCH`, the `IS*` predicates, `NA`/`N`/`TYPE`/`ERROR.TYPE`. |
| **3 — text** | Substring/case/trim, `SUBSTITUTE`/`REPLACE`/`FIND`/`SEARCH`, `TEXT`/`VALUE`, `TEXTJOIN`/`SPLIT`/`JOIN`, `REGEXMATCH`/`REGEXEXTRACT`/`REGEXREPLACE`. |
| **4 — date & time** | `DATE`/`TIME`/`TODAY`/`NOW`, decoders, `EDATE`/`EOMONTH`/`DATEDIF`/`WEEKDAY`/`WORKDAY`/`NETWORKDAYS`/`YEARFRAC`. |
| **5 — lookup & reference** | `VLOOKUP`/`HLOOKUP`/`XLOOKUP`/`LOOKUP`, `INDEX`/`MATCH`/`XMATCH`, `CHOOSE`/`OFFSET`/`INDIRECT`/`ADDRESS`, `ROW`/`COLUMN`/`ROWS`/`COLUMNS`/`TRANSPOSE`. |
| **6 — statistical** | `AVERAGEIF(S)`/`COUNT*`/`MIN/MAXIFS`, `MEDIAN`/`MODE`/`STDEV*`/`VAR*`, `LARGE`/`SMALL`/`RANK`/`PERCENTILE`/`QUARTILE`/`CORREL`. |
| **7 — dynamic arrays** | `FILTER`/`SORT`/`SORTN`/`UNIQUE`/`SEQUENCE`/`FLATTEN` (spilling, R4). |
| **8 — financial (basic)** | `PMT`/`FV`/`PV`/`NPV`/`IRR`/`RATE`/`NPER`/`IPMT`/`PPMT`/`SLN`/`DB`/`DDB`. |

## Compatibility inventory

Legend: **done** · **planned** · **out** (see ADR 0004 out-of-scope). Origin: **E** Excel · **S** Sheets-only. The current implementation ([builtins.go](../../internal/sheet/builtins.go)) has `SUM MIN MAX COUNT AVG ABS ROUND CONCAT LEN IF`; `AVG` is retained as an alias of `AVERAGE`. All entries are **planned** at Phase 0 until their phase lands.

### Math & trig (Phase 1) — E

`ABS SIGN INT TRUNC ROUND ROUNDUP ROUNDDOWN MROUND CEILING FLOOR MOD QUOTIENT POWER SQRT SQRTPI EXP LN LOG LOG10 PI SIN COS TAN ASIN ACOS ATAN ATAN2 SINH COSH TANH DEGREES RADIANS GCD LCM FACT FACTDOUBLE COMBIN PERMUT PRODUCT SUMPRODUCT SUMSQ SUMIF SUMIFS`

### Logical & information (Phase 2) — E

`IF IFS IFERROR IFNA AND OR NOT XOR SWITCH TRUE FALSE ISBLANK ISERROR ISERR ISNA ISNUMBER ISTEXT ISNONTEXT ISLOGICAL ISREF ISEVEN ISODD NA N TYPE ERROR.TYPE`

### Text (Phase 3) — E, plus S: `SPLIT JOIN REGEXMATCH REGEXEXTRACT REGEXREPLACE`

`CONCAT CONCATENATE TEXTJOIN LEFT RIGHT MID LEN LOWER UPPER PROPER TRIM CLEAN SUBSTITUTE REPLACE FIND SEARCH TEXT VALUE NUMBERVALUE REPT CHAR CODE UNICHAR UNICODE EXACT T FIXED`

### Date & time (Phase 4) — E

`DATE TIME TODAY NOW YEAR MONTH DAY HOUR MINUTE SECOND WEEKDAY WEEKNUM ISOWEEKNUM EDATE EOMONTH DATEDIF DAYS DAYS360 NETWORKDAYS WORKDAY DATEVALUE TIMEVALUE YEARFRAC`

### Lookup & reference (Phase 5) — E

`VLOOKUP HLOOKUP XLOOKUP LOOKUP INDEX MATCH XMATCH CHOOSE OFFSET INDIRECT ADDRESS ROW COLUMN ROWS COLUMNS TRANSPOSE`

### Statistical (Phase 6) — E

`AVERAGE AVERAGEA AVERAGEIF AVERAGEIFS COUNT COUNTA COUNTBLANK COUNTIF COUNTIFS MINIFS MAXIFS MEDIAN MODE STDEV STDEVP VAR VARP LARGE SMALL RANK PERCENTILE QUARTILE CORREL SLOPE INTERCEPT`

### Dynamic arrays (Phase 7) — S/E

`FILTER SORT SORTN UNIQUE SEQUENCE FLATTEN`

### Financial (Phase 8) — E

`PMT FV PV NPV IRR RATE NPER IPMT PPMT SLN DB DDB`

### Out of scope (ADR 0004)

Network/import (`GOOGLEFINANCE IMPORTRANGE IMPORTHTML IMPORTXML IMPORTDATA IMPORTFEED WEBSERVICE`), `QUERY`, cross-sheet references, cell number-format state, `LAMBDA`/named-function macros, pivot and database (`D*`) functions.
