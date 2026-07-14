# ADR 0003: chosen semantics for SPECIFICATION [open] items

## Status

Accepted (2026-07-14). These are **implementation choices** layered over the grammar; [SPECIFICATION.md](https://github.com/uplang/tsvsheet/blob/main/SPECIFICATION.md) deliberately leaves them open and is not amended by this repo.

## The §8 worked example is not a conformance target

The 2011 expected output in SPEC §8 is internally inconsistent under every consistent reading of §5 (e.g. row 3 of `F` computes to a number, not `#REF!`, under the same reading that makes rows 1–2 `#REF!`; no row base — 0- or 1-based, header-inclusive or not — makes `IF(A,C$3,D$3)` yield the shown `4`; the final `104` is unreachable from `sum($B$1:$F$-1)`). SPEC §8 itself flags the propagation rule as [open]. Conformance is therefore defined by this ADR's rules and this repo's golden tests, with the discrepancy recorded here rather than a test bent to match an inconsistent example.

## Chosen rules

1. **Row base.** Data rows are 1-based in absolute references (`C$4` is the 4th data row); header rows are not addressable. `C1` is one row before current, `C0`/`C` the current row, `C+1` one after — per §5.2.
2. **Out-of-grid → `#REF!` for operands; placement targets may extend.** A reference used as an expression **operand** (or range/matrix endpoint) that resolves outside the current grid yields the error value `#REF!`, no clamping. A reference used as a **placement target** (left of the `=` payload separator, or a structural command's selection) may address one past the last row/column (`*$+1`, `[,$+1]`, `A$+1`) — that *appends* per §5.2; any farther out is a `check` diagnostic and the placement is skipped at compute time.
3. **Strict propagation.** Any expression or aggregate with a `#REF!` operand is `#REF!`. `if(cond, a, b)` evaluates lazily: only `cond` and the selected branch propagate.
4. **Mixed matrix endpoints** (`C1:E$`): each endpoint resolves independently (relative against the current row, absolute against the grid); the matrix is the rectangular hull of the two resolved cells. If either endpoint is `#REF!`, the matrix is `#REF!`.
5. **Forward references** (`C+1`, `B+2`): resolve against the **raw data grid** for rows at or after the current row (computed values exist only for already-processed rows). There is no fixpoint recalculation — §9's single top-to-bottom pass is preserved.
6. **Self-and-rightward references in a row** (`=sum(A:H)` where `H` is the cell being computed): cells in the current row that are not yet computed contribute their raw data value, or empty (excluded from aggregates) when no raw value exists. A cell never observes its own computed value.
7. **Range-scoped structural modifiers** (`C:E<`, `C:E>`, `C:E!`): **rejected** with a diagnostic naming SPEC §6 [open]. Single-column/cell/row structural operations are implemented; the range forms are unspecified and refusing them beats inventing silent semantics.
8. **Empty cells and types.** An empty cell is the empty string. In aggregates an empty cell is **excluded** — it contributes nothing to `sum`/`avg`/`min`/`max` and is not counted by `count` (one rule, no 0-coercion; this supersedes any reading under which an included empty is 0). Direct arithmetic on an empty cell treats it as 0 (`=A + 1` with empty `A` is 1); comparisons/arithmetic on a non-numeric, non-empty string yield `#VALUE!` (a second error value, propagating like `#REF!`).
9. **Truthiness** for `if`: a number is true iff non-zero; a string is true iff non-empty; error values propagate.
10. **Function set** (case-insensitive): `sum`, `min`, `max`, `count`, `avg`, `abs`, `round`, `if`, `concat`, `len`. Unknown functions are a `check` diagnostic and evaluate to `#NAME?` (third error value, same propagation).
11. **Literal with interior spaces** (lexer [open]): the lexer skips spaces, so a placed literal (`A$+1=Total`) is a single bareword/number/string; multi-word literals require a quoted string. Data cells in `.tsv` are untouched — the limitation applies to `.tsvt` payloads only.
12. **Comparison semantics** (§11 [note]): comparisons yield the numbers 1 (true) / 0 (false); numeric comparison when both operands are numeric, lexicographic for strings, `#VALUE!` for mixed.
13. **Standalone structural commands vs cell modifiers.** SPEC §4/§8 describe `=A<` as "insert a column before column A" while §6's cell-position `C<` swaps columns B and C — a genuine contradiction the SPEC does not mark. Pinned: in a **standalone** command (`=ref` + modifier as the whole first field), `<` inserts a new empty column/row *before* the selection and `>` inserts *after* (the selection and everything beyond shifts away from the insertion), `!` deletes. In **cell position**, §6 applies verbatim: `>` shifts the neighbor away (leaving the vacated cell empty), `<` swaps with the left/up neighbor, `!` deletes and closes the gap.
14. **Division and modulo** (§11.4 [note]): `x/0` and `x%0` yield `#DIV/0!` (a fourth error value, propagating like `#REF!`); `%` is IEEE remainder truncated toward zero (Go `math.Mod`), defined for negative and fractional operands.
15. **Named-column reference without a header** (`"Sum"` when no `=header(n)` bound it): a `check` diagnostic, and at compute time the reference evaluates to `#NAME?`.
