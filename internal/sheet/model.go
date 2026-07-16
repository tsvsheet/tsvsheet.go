// Package sheet is the tsvsheet spreadsheet engine: a .tsvt file IS the sheet —
// a TAB-separated grid whose cells are literal values or `=formulas`, and whose
// formulas address other cells by A1 reference (`B2`, `D2:D4`) exactly like a
// conventional spreadsheet. It parses the grid, evaluates every formula in
// dependency order (memoized, with cycle detection), and emits the computed
// grid. The expression sublanguage and its AST come from internal/tsvt (the
// ANTLR-generated parser); this package resolves A1 references and evaluates.
package sheet

import (
	"bytes"
	"strings"
	"time"

	"github.com/uplang/tsvsheet.go/internal/constants"
	"github.com/uplang/tsvsheet.go/internal/tsvt"
)

// cell is one spreadsheet cell: a verbatim literal, or a compiled formula.
type cell struct {
	formula tsvt.Expr
	text    string
}

// isFormula reports whether the cell holds a formula.
func (c cell) isFormula() boolResult { return boolResult(c.formula != nil) }

// CellInfo describes one non-empty cell: its address, source text, and whether
// it is a formula — the projection the parse command emits.
type CellInfo struct {
	Text      string
	Address   Address
	IsFormula bool
}

// Sheet is a parsed spreadsheet grid of literal and formula cells.
type Sheet struct {
	cells [][]cell
}

// formulaMarker is the leading byte that makes a cell a formula.
const formulaMarker = "="

// Parse reads a .tsvt grid: each TAB-separated field is a literal, or — when it
// begins with `=` — a formula compiled from the expression that follows. A
// malformed formula is a syntax error naming its cell.
func Parse(src []byte) (Sheet, error) {
	grid, err := ReadTSV(bytes.NewReader(src))
	if err != nil {
		return Sheet{}, err
	}
	cells := make([][]cell, len(grid))
	for r, row := range grid {
		cells[r] = make([]cell, len(row))
		for c, text := range row {
			parsed, cellErr := parseCell(textVal(text), rowIndex(r), colIndex(c))
			if cellErr != nil {
				return Sheet{}, cellErr
			}
			cells[r][c] = parsed
		}
	}
	return Sheet{cells: cells}, nil
}

// parseCell classifies a field as a literal or compiles its formula, naming the
// cell (in A1 notation) on a formula syntax error.
func parseCell(text textVal, row rowIndex, col colIndex) (cell, error) {
	if !strings.HasPrefix(string(text), formulaMarker) {
		return cell{text: string(text)}, nil
	}
	expr, err := tsvt.ParseFormula(tsvt.FormulaText(text[1:]))
	if err != nil {
		at := Address{Row: int(row), Col: int(col)}
		return cell{}, constants.ErrSyntax.With(err, "cell", at.String())
	}
	return cell{text: string(text), formula: expr}, nil
}

// Compute evaluates every formula in dependency order and returns the value
// grid: literal cells pass through verbatim, formula cells are replaced by their
// computed value. Volatile functions (TODAY/NOW) sample the wall clock once for
// the whole pass.
func (s Sheet) Compute() Grid { return s.ComputeAt(time.Now()) }

// ComputeAt is Compute with the clock injected, so volatile functions are
// deterministic within a pass (and testable). It computes every cell's value,
// then renders — spilling dynamic-array results into empty neighbours.
func (s Sheet) ComputeAt(at time.Time) Grid {
	return s.computeGrid(newComputer(s, at))
}

// computeGrid evaluates every cell through comp and renders the value grid,
// spilling any dynamic-array results. It is the shared body of ComputeAt (plain)
// and ComputeWith (with an embedded-sheet loader).
func (s Sheet) computeGrid(comp computer) Grid {
	values := make([][]Value, len(s.cells))
	for r, row := range s.cells {
		values[r] = make([]Value, len(row))
		for c, cl := range row {
			values[r][c] = comp.cellValue(rowIndex(r), colIndex(c), cl)
		}
	}
	return s.render(values)
}

// dims is an output grid's row and column extent.
type dims struct{ rows, cols int }

// render turns the computed value grid into the output string grid, spilling any
// dynamic-array result down-and-right from its anchor.
func (s Sheet) render(values [][]Value) Grid {
	out := s.fillScalars(values, outputExtent(values))
	s.spillArrays(out, values)
	return out
}

// outputExtent is the grid extent needed once every array result has spilled.
func outputExtent(values [][]Value) dims {
	rows, cols := len(values), 0
	for r := range values {
		cols = max(cols, len(values[r]))
		for c := range values[r] {
			if a := values[r][c]; a.kind == kindArray {
				rows = max(rows, r+len(a.arr))
				cols = max(cols, c+len(a.arr[0]))
			}
		}
	}
	return dims{rows: rows, cols: cols}
}

// fillScalars renders each cell at its own position: a literal verbatim, a
// formula's computed value (an array renders its top-left anchor here;
// spillArrays overwrites the spilled cells).
func (s Sheet) fillScalars(values [][]Value, d dims) Grid {
	out := make(Grid, d.rows)
	for r := range out {
		out[r] = make([]string, d.cols)
		for c := range out[r] {
			out[r][c] = s.scalarText(values, Address{Row: r, Col: c})
		}
	}
	return out
}

// scalarText renders one cell's own text: a literal verbatim, a formula's value,
// empty when beyond the source grid.
func (s Sheet) scalarText(values [][]Value, at Address) string {
	if at.Row >= len(values) || at.Col >= len(values[at.Row]) {
		return ""
	}
	if !s.cells[at.Row][at.Col].isFormula() {
		return s.cells[at.Row][at.Col].text
	}
	return values[at.Row][at.Col].String()
}

// spillArrays writes each array result into the output grid.
func (s Sheet) spillArrays(out Grid, values [][]Value) {
	for r := range values {
		for c := range values[r] {
			if values[r][c].kind == kindArray {
				s.spill(out, Address{Row: r, Col: c}, values[r][c].arr)
			}
		}
	}
}

// spill writes an array from its anchor, or #SPILL! at the anchor when a target
// cell is occupied.
func (s Sheet) spill(out Grid, anchor Address, arr [][]Value) {
	if s.spillBlocked(anchor, arr) {
		out[anchor.Row][anchor.Col] = string(ErrSpill)
		return
	}
	for i := range arr {
		for j := range arr[i] {
			out[anchor.Row+i][anchor.Col+j] = arr[i][j].String()
		}
	}
}

// spillBlocked reports whether any non-anchor target cell already holds content.
func (s Sheet) spillBlocked(anchor Address, arr [][]Value) boolResult {
	for i := range arr {
		for j := range arr[i] {
			target := Address{Row: anchor.Row + i, Col: anchor.Col + j}
			if s.blocksSpill(anchor, target) {
				return true
			}
		}
	}
	return false
}

// blocksSpill reports whether target (a spill destination other than the anchor
// itself) already holds content and so blocks the spill.
func (s Sheet) blocksSpill(anchor, target Address) boolResult {
	return boolResult(target != anchor && !bool(s.isEmptyCell(target)))
}

// isEmptyCell reports whether a source cell is empty (spillable): out of the
// source grid, or a blank non-formula cell.
func (s Sheet) isEmptyCell(at Address) boolResult {
	if at.Row >= len(s.cells) || at.Col >= len(s.cells[at.Row]) {
		return true
	}
	cl := s.cells[at.Row][at.Col]
	return boolResult(cl.text == "" && !bool(cl.isFormula()))
}

// at returns the cell at (row, col); the boolean reports whether the position
// is within the grid.
func (s Sheet) at(row rowIndex, col colIndex) (cell, boolResult) {
	if row < 0 || int(row) >= len(s.cells) || col < 0 || int(col) >= len(s.cells[row]) {
		return cell{}, false
	}
	return s.cells[row][col], true
}

// Cells returns every non-empty cell of the sheet as CellInfo, in row-major
// order.
func (s Sheet) Cells() []CellInfo {
	var out []CellInfo
	for r, row := range s.cells {
		for c, cl := range row {
			if cl.text == "" {
				continue
			}
			out = append(out, CellInfo{
				Address:   Address{Row: r, Col: c},
				Text:      cl.text,
				IsFormula: bool(cl.isFormula()),
			})
		}
	}
	return out
}

// Source returns the sheet's cell source texts (literals and "=formulas") as a
// grid — what an editor shows and what is saved back to the .tsvt file.
func (s Sheet) Source() Grid {
	out := make(Grid, len(s.cells))
	for r, row := range s.cells {
		out[r] = make([]string, len(row))
		for c, cl := range row {
			out[r][c] = cl.text
		}
	}
	return out
}

// Set returns a new sheet with the cell at addr replaced by text (a literal or
// a formula), growing the grid to reach an out-of-bounds position. The injected
// limits bound how far the grid may grow. A malformed formula is a syntax error
// and the sheet is unchanged (Set is immutable, so the caller simply keeps the
// old value).
func (s Sheet) Set(addr Address, text string, limits Limits) (Sheet, error) {
	if addr.Row < 0 || addr.Col < 0 {
		return Sheet{}, constants.ErrInvalidValue.With(nil, "address", addr.String())
	}
	if addr.Row >= limits.GridDim || addr.Col >= limits.GridDim {
		return Sheet{}, constants.ErrInvalidValue.With(nil, "address exceeds the grid limit", addr.String())
	}
	parsed, err := parseCell(textVal(text), rowIndex(addr.Row), colIndex(addr.Col))
	if err != nil {
		return Sheet{}, err
	}
	cells := growCells(s.cells, addr)
	cells[addr.Row][addr.Col] = parsed
	return Sheet{cells: cells}, nil
}

// growCells deep-copies the cell grid, extending it with empty rows and — in
// the target row only — empty cells, so at is addressable.
func growCells(src [][]cell, at Address) [][]cell {
	out := make([][]cell, max(len(src), at.Row+1))
	for r := range src {
		out[r] = append([]cell(nil), src[r]...)
	}
	for len(out[at.Row]) <= at.Col {
		out[at.Row] = append(out[at.Row], cell{})
	}
	return out
}
