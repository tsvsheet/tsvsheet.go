package sheet

import "github.com/uplang/tsvsheet.go/internal/tsvt"

// Span is a rectangular reference target resolved to 0-based addresses: a single
// cell (From == To) or a range (From is the top-left, To the bottom-right as
// written). It is the projection a frontend highlights.
type Span struct {
	From Address `json:"from"`
	To   Address `json:"to"`
}

// contains reports whether the span's rectangle covers at, normalising the
// corners so an unordered range (e.g. B2:A1) still matches.
func (sp Span) contains(at Address) boolResult {
	inRows := at.Row >= min(sp.From.Row, sp.To.Row) && at.Row <= max(sp.From.Row, sp.To.Row)
	inCols := at.Col >= min(sp.From.Col, sp.To.Col) && at.Col <= max(sp.From.Col, sp.To.Col)
	return boolResult(inRows && inCols)
}

// Precedents returns the cell and range references the formula at `at` reads,
// as resolved spans in source order. A literal cell, an address off the grid,
// or a formula with no references returns nil.
func (s Sheet) Precedents(at Address) []Span {
	cl, inGrid := s.at(rowIndex(at.Row), colIndex(at.Col))
	if !inGrid || !cl.isFormula() {
		return nil
	}
	var spans []Span
	walkRefs(cl.formula, func(ref tsvt.Reference) {
		if span, ok := refSpan(ref); ok {
			spans = append(spans, span)
		}
	})
	return spans
}

// refSpan resolves an A1 reference to a 0-based span; ok is false when either
// endpoint is a malformed address (a row-0 reference the grammar admits but the
// flat grid has no cell for).
func refSpan(ref tsvt.Reference) (Span, boolResult) {
	rangeRef := ref.(tsvt.RangeRef)
	if rangeRef.File != "" {
		return Span{}, false // a cross-sheet reference is not a local span
	}
	from, ok := a1Address(rangeRef.From)
	if !ok {
		return Span{}, false
	}
	to := from
	if rangeRef.To != nil {
		var toOK boolResult
		if to, toOK = a1Address(*rangeRef.To); !toOK {
			return Span{}, false
		}
	}
	return Span{From: from, To: to}, true
}

// Dependents returns every formula cell whose references cover `at`, in
// row-major order — the reverse edge of Precedents.
func (s Sheet) Dependents(at Address) []Address {
	var deps []Address
	s.eachFormula(func(cell Address) {
		if covers(s.Precedents(cell), at) {
			deps = append(deps, cell)
		}
	})
	return deps
}

// covers reports whether any span contains at.
func covers(spans []Span, at Address) boolResult {
	for _, span := range spans {
		if span.contains(at) {
			return true
		}
	}
	return false
}

// eachFormula visits the address of every formula cell in row-major order.
func (s Sheet) eachFormula(visit func(Address)) {
	for r, row := range s.cells {
		for c, cl := range row {
			if cl.isFormula() {
				visit(Address{Row: r, Col: c})
			}
		}
	}
}
