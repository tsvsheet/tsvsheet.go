package sheet

import (
	"strings"

	"github.com/uplang/tsvsheet.go/internal/tsvt"
)

// The content-typed import media types (ADR 0006 §2): the request Accept header
// each IMPORT* function sends, which the response Content-Type must match. The
// RFC 6838 vendor tree with a hierarchical subtype for granularity and the +tsv
// structured-syntax suffix. The values are consumed when Phase 1 injects a
// Fetcher; Phase 0 uses only the key set (isImportName).
const (
	mediaSheet  = "application/vnd.tsvsheet+tsv"
	mediaCell   = "application/vnd.tsvsheet.cell+tsv"
	mediaRow    = "application/vnd.tsvsheet.row+tsv"
	mediaColumn = "application/vnd.tsvsheet.column+tsv"
	mediaRange  = "application/vnd.tsvsheet.range+tsv"
)

// importMedia maps each lowercase import function name to the media type it
// requests — the name is the content type (ADR 0006 §2).
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

// evalImport dispatches the five IMPORT* functions. Phase 0 is the disabled
// seam: with no injected Fetcher every import is #IMPORT! (ADR 0006 §4). Phase 1
// replaces the body with the arity check, fetch, content-type handshake, and
// values-only parse; the evalLazy wiring is unchanged. ok is false for any
// non-import name.
func (r resolver) evalImport(name funcName, _ []tsvt.Expr) (Value, boolResult) {
	if !isImportName(name) {
		return Value{}, false
	}
	return errorValue(ErrImport), true
}
