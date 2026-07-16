# Import Engine Seam (Phase 0) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land the recognizable-but-disabled engine seam for content-typed imports — the `#IMPORT!` error value, the five `IMPORT*` function names mapped to their media types, a `HasImports` predicate, and dispatch that yields `#IMPORT!` while no `Fetcher` is injected.

**Architecture:** Purely additive engine work in `internal/sheet`, entirely test-covered with no network and no frontend changes. The five import functions are recognized by the existing lazy-dispatch chain and resolve to `#IMPORT!` (the feature is off until Phase 1 injects a `Fetcher`). This mirrors how a `nil` `Loader` makes `SHEET()` resolve to `#REF!` today.

**Tech Stack:** Go, standard `testing` with `stretchr/testify` (permitted by the capability spec), the gomatic quality gate (`make check`).

> **Status: complete** (commit `30630a8`). Implemented via the external `package sheet_test` (all repo tests are external, asserting through the public `HasImports()`/`Compute` surface rather than the unexported symbols shown in the tasks below). New code builds, is staticcheck-clean, and is 100%-covered by three tests. The repo-wide `make check` is pre-existingly red (floated `yze`/`stickler` findings in `limits.go`/`app.go`/`tui/grid.go`, untouched here) — a separate modernization task.

## Global Constraints

_Copied verbatim from [specs/capabilities/import.md](../capabilities/import.md) and [ADR 0006](../decisions/0006-content-typed-import.md); every task's requirements implicitly include these._

- Errors are `errs.Const` sentinels in `internal/constants`; never `fmt.Errorf`/`errors.New`.
- Value receivers except `session.Session` (unchanged here — all new methods are value receivers).
- `make check` green after every task: gofumpt, `go vet` (grammar-excluded), staticcheck (no unused symbols — U1000), golangci (gocognit ≤ 7), govulncheck, **100.0% aggregate coverage**.
- Custom named types for domain parameters (`type ImportURL string`, `type MediaType string`) — introduced in Phase 1 where they are first used, not here.
- Media types (for reference; used as map values in Task 2): `application/vnd.tsvsheet+tsv` (sheet), `application/vnd.tsvsheet.cell+tsv`, `.row+tsv`, `.column+tsv`, `.range+tsv`.
- No grammar change — the five names are ordinary function calls the grammar already admits.
- **Scope boundary:** Phase 0 does NOT add the `Fetcher` interface, the `computer.fetcher` field, `ComputeOptions.Fetcher`, real fetching, handshake, values-only parsing, or `explain` detail. Those are Phase 1+ (a follow-on plan), because an unreferenced `Fetcher` field/type would fail staticcheck U1000. Phase 0's deliverable is: the names are recognized and resolve to `#IMPORT!`, and `HasImports` reports them.

---

### Task 1: The `#IMPORT!` error value

**Files:**
- Modify: `internal/sheet/value.go:16-26` (add the constant), `internal/sheet/value.go:96-103` (add to `isErrorCode`)
- Test: `internal/sheet/value_test.go` (add cases; create if absent)

**Interfaces:**
- Consumes: nothing.
- Produces: `ErrImport ErrorValue = "#IMPORT!"` — the cell-level error every import failure surfaces as; round-trips through `value()`/`isErrorCode` like the other error values.

- [ ] **Step 1: Write the failing test**

Add to `internal/sheet/value_test.go`:

```go
func TestImportErrorValueRoundTrips(t *testing.T) {
	require.True(t, isErrorCode("#IMPORT!"), "#IMPORT! must be a recognized error code")
	v := value("#IMPORT!")
	require.True(t, v.isError(), "value(\"#IMPORT!\") must be an error value")
	require.Equal(t, "#IMPORT!", v.String())
	require.Equal(t, ErrImport, ErrorValue(v.String()))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/sheet/ -run TestImportErrorValueRoundTrips -v`
Expected: FAIL — `undefined: ErrImport` (and `isErrorCode` returns false).

- [ ] **Step 3: Write minimal implementation**

In `internal/sheet/value.go`, add `ErrImport` to the error-value block (after `ErrSpill`):

```go
	ErrSpill  ErrorValue = "#SPILL!"
	ErrImport ErrorValue = "#IMPORT!"
```

Update the block comment above it to mention `#IMPORT!` (import failure). Then add `ErrImport` to the `isErrorCode` switch:

```go
	case ErrRef, ErrValue, ErrName, ErrDiv, ErrCirc, ErrNA, ErrNum, ErrNull, ErrSpill, ErrImport:
		return true
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/sheet/ -run TestImportErrorValueRoundTrips -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/sheet/value.go internal/sheet/value_test.go
git commit -m "feat(sheet): add the #IMPORT! error value"
```

---

### Task 2: Import function names → media types

**Files:**
- Create: `internal/sheet/import.go`
- Test: `internal/sheet/import_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `importMedia map[string]string` — the five lowercase import function names mapped to their media-type strings. Consumed by `HasImports` (Task 3) and `evalImport` (Task 4).
  - `isImportName(name funcName) boolResult` — reports whether a (lowercased) function name is an import.

- [ ] **Step 1: Write the failing test**

Create `internal/sheet/import_test.go`:

```go
package sheet

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImportMediaCoversFiveGranularities(t *testing.T) {
	require.Equal(t, map[string]string{
		"importcell":   "application/vnd.tsvsheet.cell+tsv",
		"importrow":    "application/vnd.tsvsheet.row+tsv",
		"importcolumn": "application/vnd.tsvsheet.column+tsv",
		"importrange":  "application/vnd.tsvsheet.range+tsv",
		"importsheet":  "application/vnd.tsvsheet+tsv",
	}, importMedia)
}

func TestIsImportName(t *testing.T) {
	require.True(t, bool(isImportName("importcell")))
	require.True(t, bool(isImportName("importsheet")))
	require.False(t, bool(isImportName("sum")))
	require.False(t, bool(isImportName("import")))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/sheet/ -run 'TestImportMedia|TestIsImportName' -v`
Expected: FAIL — `undefined: importMedia`, `undefined: isImportName`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/sheet/import.go`:

```go
package sheet

// The content-typed import media types (ADR 0006 §2): the request Accept header
// each IMPORT* function sends, which the response Content-Type must match. The
// RFC 6838 vendor tree with a hierarchical subtype for granularity and the +tsv
// structured-syntax suffix.
const (
	mediaSheet  = "application/vnd.tsvsheet+tsv"
	mediaCell   = "application/vnd.tsvsheet.cell+tsv"
	mediaRow    = "application/vnd.tsvsheet.row+tsv"
	mediaColumn = "application/vnd.tsvsheet.column+tsv"
	mediaRange  = "application/vnd.tsvsheet.range+tsv"
)

// importMedia maps each lowercase import function name to the media type it
// requests. The name is the content type (ADR 0006 §2).
var importMedia = map[string]string{
	"importcell":   mediaCell,
	"importrow":    mediaRow,
	"importcolumn": mediaColumn,
	"importrange":  mediaRange,
	"importsheet":  mediaSheet,
}

// isImportName reports whether name (already lowercased) is an import function.
func isImportName(name funcName) boolResult {
	_, ok := importMedia[string(name)]
	return boolResult(ok)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/sheet/ -run 'TestImportMedia|TestIsImportName' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/sheet/import.go internal/sheet/import_test.go
git commit -m "feat(sheet): map the five IMPORT* names to their media types"
```

---

### Task 3: `HasImports` predicate

**Files:**
- Modify: `internal/sheet/import.go` (add the method)
- Test: `internal/sheet/import_test.go` (add cases)

**Interfaces:**
- Consumes: `isImportName` (Task 2); `Sheet.eachFormula` and `walkCalls` (existing, used by `IsVolatile` in `internal/sheet/volatile.go`).
- Produces: `func (s Sheet) HasImports() bool` — reports whether any formula calls an `IMPORT*` function, so a frontend can expose a refresh control WITHOUT placing imports on the isnow ticker (ADR 0006 §6). Deliberately separate from `IsVolatile`.

- [ ] **Step 1: Write the failing test**

Add to `internal/sheet/import_test.go`:

```go
func TestHasImports(t *testing.T) {
	with, err := Parse([]byte("=IMPORTCELL(\"https://x.example/v\")"))
	require.NoError(t, err)
	require.True(t, with.HasImports())

	without, err := Parse([]byte("=SUM(1,2)"))
	require.NoError(t, err)
	require.False(t, without.HasImports())

	literal, err := Parse([]byte("hello"))
	require.NoError(t, err)
	require.False(t, literal.HasImports())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/sheet/ -run TestHasImports -v`
Expected: FAIL — `with.HasImports undefined`.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/sheet/import.go` (mirroring `IsVolatile` in `internal/sheet/volatile.go`):

```go
import (
	"strings"

	"github.com/uplang/tsvsheet.go/internal/tsvt"
)

// HasImports reports whether any formula calls an IMPORT* function, so a
// frontend can offer a manual "refresh imports" control. Imports are NOT
// clock-volatile and are deliberately absent from IsVolatile — they must never
// ride the isnow refresh ticker (ADR 0006 §6).
func (s Sheet) HasImports() bool {
	found := false
	s.eachFormula(func(at Address) {
		walkCalls(s.cells[at.Row][at.Col].formula, func(call tsvt.Call) {
			if isImportName(funcName(strings.ToLower(call.Name))) {
				found = true
			}
		})
	})
	return found
}
```

(Move the `const`/`var` from Task 2 below the new `import (...)` block, or add the imports at the top of the file — keep one import block per Go file. `gofumpt` will order it; run `make fmt` if needed.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/sheet/ -run TestHasImports -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/sheet/import.go internal/sheet/import_test.go
git commit -m "feat(sheet): HasImports predicate, distinct from IsVolatile"
```

---

### Task 4: Disabled dispatch — IMPORT* resolves to `#IMPORT!`

**Files:**
- Modify: `internal/sheet/import.go` (add `evalImport`), `internal/sheet/funcs.go:103-123` (`evalLazy`: add the import case)
- Test: `internal/sheet/import_test.go` (add cases)

**Interfaces:**
- Consumes: `importMedia`/`isImportName` (Task 2); `ErrImport` (Task 1); the `resolver` type and `evalLazy` dispatch chain (existing, `internal/sheet/funcs.go`).
- Produces: `func (r resolver) evalImport(name funcName, args []tsvt.Expr) (Value, boolResult)` — returns `(#IMPORT!, true)` for an import name, `(Value{}, false)` otherwise. Phase 1 replaces the body with arity check, fetch, handshake, and values-only parse; the dispatch wiring stays.

- [ ] **Step 1: Write the failing test**

Add to `internal/sheet/import_test.go`:

```go
func TestImportDisabledYieldsImportError(t *testing.T) {
	for _, src := range []string{
		"=IMPORTCELL(\"https://x.example/v\")",
		"=IMPORTROW(\"https://x.example/v\")",
		"=IMPORTCOLUMN(\"https://x.example/v\")",
		"=IMPORTRANGE(\"https://x.example/v\")",
		"=IMPORTSHEET(\"https://x.example/v\")",
	} {
		s, err := Parse([]byte(src))
		require.NoError(t, err)
		got := s.ComputeAt(time.Unix(0, 0))
		require.Equal(t, "#IMPORT!", got[0][0], "no Fetcher injected: %s must be #IMPORT!", src)
	}
}
```

Add `"time"` to the test file's imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/sheet/ -run TestImportDisabledYieldsImportError -v`
Expected: FAIL — the call is unknown, so it currently yields `#NAME?`, not `#IMPORT!`.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/sheet/import.go`:

```go
// evalImport dispatches the five IMPORT* functions. Phase 0 is the disabled
// seam: with no injected Fetcher every import is #IMPORT! (ADR 0006 §4). Phase 1
// replaces the body with the arity check, fetch, content-type handshake, and
// values-only parse; the dispatch wiring in evalLazy is unchanged. ok is false
// for any non-import name.
func (r resolver) evalImport(name funcName, args []tsvt.Expr) (Value, boolResult) {
	if !isImportName(name) {
		return Value{}, false
	}
	return errorValue(ErrImport), true
}
```

Then wire it into `evalLazy` in `internal/sheet/funcs.go` — add the import case before the final inspector fallthrough:

```go
	if v, ok := r.evalEmbed(name, args); ok {
		return v, true
	}
	if v, ok := r.evalImport(name, args); ok {
		return v, true
	}
	return r.evalInspector(name, args)
```

Note: `args` is unused in the Phase 0 body. To satisfy the linter without a placeholder, name it `_ []tsvt.Expr` in the signature: `func (r resolver) evalImport(name funcName, _ []tsvt.Expr) (Value, boolResult)`. Phase 1 restores the `args` name when it consumes them.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/sheet/ -run TestImportDisabledYieldsImportError -v`
Expected: PASS.

- [ ] **Step 5: Run the full gate**

Run: `make check`
Expected: green — gofumpt, vet, staticcheck (no U1000; every new symbol is referenced), golangci (gocognit ≤ 7), govulncheck, 100.0% coverage.

- [ ] **Step 6: Commit**

```bash
git add internal/sheet/import.go internal/sheet/funcs.go internal/sheet/import_test.go
git commit -m "feat(sheet): recognize IMPORT* names, resolve to #IMPORT! while disabled"
```

---

## What Phase 1 adds (next plan, not this one)

For the reviewer's context — explicitly out of scope here: the `Fetcher` interface + `ImportURL`/`MediaType` named types + `FetchResult`; the `computer.fetcher` field and `ComputeOptions.Fetcher` wiring (with a nil check re-added to `evalImport`); the URL-argument evaluation, the fetch, the `Accept`↔`Content-Type` handshake, the values-only `.tsvt`-fragment parse into the five return shapes, the strict-shape and failure→`#IMPORT!` paths, and the `explain` detail. The real `net/http` `Fetcher`, the operator allowlist, the cross-pass cache, and the frontend flags/refresh controls follow in later plans (capability spec Phases 3–4).

## Self-Review

- **Spec coverage (Phase 0 slice of [import.md](../capabilities/import.md)):** R5's `#IMPORT!` value → Task 1; R1's five names + R2's media types → Task 2; R6's `HasImports`/not-in-`IsVolatile` → Task 3; the "nil ⇒ `#IMPORT!`" half of R4 → Task 4. R2's handshake, R3 values-only, R4's `Fetcher`, R7 gate, R8 frontends, R9 failure-path tests are deferred to later plans and named in "What Phase 1 adds."
- **Placeholder scan:** none — every step shows the actual code, exact commands, and expected output.
- **Type consistency:** `importMedia` (map[string]string), `isImportName(funcName) boolResult`, `HasImports() bool`, `evalImport(funcName, []tsvt.Expr) (Value, boolResult)`, `ErrImport ErrorValue` — used consistently across Tasks 1–4 and matched to existing engine types (`funcName`, `boolResult`, `Value`, `tsvt.Call`/`tsvt.Expr`, `resolver`, `walkCalls`, `eachFormula`).
