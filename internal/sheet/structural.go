package sheet

import "github.com/uplang/tsvsheet.go/internal/tsvt"

// lineIndex is a 0-based row or column position on one axis.
type lineIndex int

// axis reads and writes a CellRef's coordinate on one dimension as a 0-based
// lineIndex, so the row and column structural edits share one implementation.
type axis struct {
	get func(tsvt.CellRef) lineIndex
	set func(tsvt.CellRef, lineIndex) tsvt.CellRef
}

var rowAxis = axis{
	get: func(c tsvt.CellRef) lineIndex { return lineIndex(c.Row - 1) },
	set: func(c tsvt.CellRef, i lineIndex) tsvt.CellRef { c.Row = int(i) + 1; return c },
}

var colAxis = axis{
	get: func(c tsvt.CellRef) lineIndex { return lineIndex(lettersToIndex(columnLetters(c.Col))) },
	set: func(c tsvt.CellRef, i lineIndex) tsvt.CellRef { c.Col = indexToLetters(colIndex(i)); return c },
}

// transform shifts reference coordinates for one structural edit: point maps a
// single-cell reference (ok=false when it was deleted), and lo/hi map a range's
// two endpoints (a range collapses to #REF! when its endpoints cross).
type transform struct {
	point func(lineIndex) (lineIndex, boolResult)
	lo    func(lineIndex) (lineIndex, boolResult)
	hi    func(lineIndex) (lineIndex, boolResult)
}

// shiftUp maps a coordinate for an insertion before `at`: coordinates at or
// after `at` move up one; nothing is ever deleted.
func shiftUp(at lineIndex) func(lineIndex) (lineIndex, boolResult) {
	return func(x lineIndex) (lineIndex, boolResult) {
		if x >= at {
			return x + 1, true
		}
		return x, true
	}
}

// insertTransform shifts every reference on the axis down by one at `at`.
func insertTransform(at lineIndex) transform {
	up := shiftUp(at)
	return transform{point: up, lo: up, hi: up}
}

// deletePoint maps a single-cell coordinate for a deletion at `at`: the deleted
// line yields ok=false (#REF!); lines after it move up one.
func deletePoint(at lineIndex) func(lineIndex) (lineIndex, boolResult) {
	return func(x lineIndex) (lineIndex, boolResult) {
		if x == at {
			return 0, false
		}
		if x > at {
			return x - 1, true
		}
		return x, true
	}
}

// deleteLo maps a range's low endpoint for a deletion: it clamps to `at` (the
// line that slides into the deleted slot), so the range's start survives.
func deleteLo(at lineIndex) func(lineIndex) (lineIndex, boolResult) {
	return func(x lineIndex) (lineIndex, boolResult) {
		if x > at {
			return x - 1, true
		}
		return x, true
	}
}

// deleteHi maps a range's high endpoint for a deletion: it clamps to `at`-1 (the
// last line before the deleted slot); combined with deleteLo, a range that was
// exactly the deleted line collapses (lo > hi → #REF!).
func deleteHi(at lineIndex) func(lineIndex) (lineIndex, boolResult) {
	return func(x lineIndex) (lineIndex, boolResult) {
		if x >= at {
			return x - 1, true
		}
		return x, true
	}
}

// deleteTransform shifts references on the axis up by one past `at`, turning a
// reference to the deleted line into #REF!.
func deleteTransform(at lineIndex) transform {
	return transform{point: deletePoint(at), lo: deleteLo(at), hi: deleteHi(at)}
}

// InsertRow returns a new sheet with a blank row inserted before at.Row; every
// reference to a row at or below it shifts down to follow its data. Only the
// row coordinate of at is used.
func (s Sheet) InsertRow(at Address) Sheet {
	return rewriteAll(
		insertLine(s.cells, rowIndex(min(at.Row, len(s.cells)))),
		rowAxis,
		insertTransform(lineIndex(at.Row)),
	)
}

// DeleteRow returns a new sheet with row at.Row removed; references to it become
// #REF! and references below it shift up. An out-of-range row is a no-op. Only
// the row coordinate of at is used.
func (s Sheet) DeleteRow(at Address) Sheet {
	if at.Row < 0 || at.Row >= len(s.cells) {
		return s
	}
	return rewriteAll(deleteLine(s.cells, rowIndex(at.Row)), rowAxis, deleteTransform(lineIndex(at.Row)))
}

// InsertCol returns a new sheet with a blank column inserted before at.Col;
// every reference to a column at or right of it shifts right. Only the column
// coordinate of at is used.
func (s Sheet) InsertCol(at Address) Sheet {
	return rewriteAll(insertColumn(s.cells, colIndex(at.Col)), colAxis, insertTransform(lineIndex(at.Col)))
}

// DeleteCol returns a new sheet with column at.Col removed; references to it
// become #REF! and references to its right shift left. A column past every row
// is a no-op. Only the column coordinate of at is used.
func (s Sheet) DeleteCol(at Address) Sheet {
	if at.Col < 0 || at.Col >= widestRow(s.cells) {
		return s
	}
	return rewriteAll(deleteColumn(s.cells, colIndex(at.Col)), colAxis, deleteTransform(lineIndex(at.Col)))
}

// insertLine splices a blank row into the grid before at. The new row is a full
// span of empty cells (as wide as the widest row) so a reference into it
// resolves to an empty cell, not an off-grid #REF!.
func insertLine(src [][]cell, at rowIndex) [][]cell {
	out := make([][]cell, 0, len(src)+1)
	out = append(out, src[:at]...)
	out = append(out, make([]cell, widestRow(src)))
	return append(out, src[at:]...)
}

// deleteLine removes row at from the grid.
func deleteLine(src [][]cell, at rowIndex) [][]cell {
	out := make([][]cell, 0, len(src)-1)
	out = append(out, src[:at]...)
	return append(out, src[at+1:]...)
}

// insertColumn splices a blank cell into every row that reaches column at.
func insertColumn(src [][]cell, at colIndex) [][]cell {
	return mapRows(src, func(row []cell) []cell {
		if int(at) > len(row) {
			return row
		}
		spliced := make([]cell, 0, len(row)+1)
		spliced = append(spliced, row[:at]...)
		spliced = append(spliced, cell{})
		return append(spliced, row[at:]...)
	})
}

// deleteColumn removes column at from every row that reaches it.
func deleteColumn(src [][]cell, at colIndex) [][]cell {
	return mapRows(src, func(row []cell) []cell {
		if int(at) >= len(row) {
			return row
		}
		return append(row[:at:at], row[at+1:]...)
	})
}

// mapRows applies f to each row, returning a new grid.
func mapRows(src [][]cell, f func([]cell) []cell) [][]cell {
	out := make([][]cell, len(src))
	for r, row := range src {
		out[r] = f(row)
	}
	return out
}

// widestRow is the column count of the widest row.
func widestRow(src [][]cell) int {
	widest := 0
	for _, row := range src {
		widest = max(widest, len(row))
	}
	return widest
}

// rewriteAll rebuilds the grid with every formula's references shifted by tr on
// the given axis.
func rewriteAll(cells [][]cell, ax axis, tr transform) Sheet {
	return Sheet{cells: mapRows(cells, func(row []cell) []cell {
		out := make([]cell, len(row))
		for c, cl := range row {
			out[c] = rewriteCell(cl, ax, tr)
		}
		return out
	})}
}

// rewriteCell shifts a formula cell's references and re-serialises it; a literal
// passes through untouched.
func rewriteCell(cl cell, ax axis, tr transform) cell {
	if !cl.isFormula() {
		return cl
	}
	shifted := mapRefs(cl.formula, func(ref tsvt.Reference) tsvt.Expr {
		return shiftReference(ref, ax, tr)
	})
	return cell{formula: shifted, text: formulaMarker + RenderExpr(shifted)}
}

// shiftReference shifts one A1 reference, returning a #REF! error literal when
// the edit deletes the cell (or collapses the range) it names.
func shiftReference(ref tsvt.Reference, ax axis, tr transform) tsvt.Expr {
	rangeRef := ref.(tsvt.RangeRef)
	if rangeRef.File != "" {
		return tsvt.RefOperand{Ref: rangeRef} // a cross-sheet ref addresses another sheet — never shift it
	}
	if rangeRef.To == nil {
		moved, ok := tr.point(ax.get(rangeRef.From))
		if !ok {
			return refError()
		}
		return tsvt.RefOperand{Ref: tsvt.RangeRef{From: ax.set(rangeRef.From, moved)}}
	}
	lo, loOK := tr.lo(ax.get(rangeRef.From))
	hi, hiOK := tr.hi(ax.get(*rangeRef.To))
	if !loOK || !hiOK || lo > hi {
		return refError()
	}
	to := ax.set(*rangeRef.To, hi)
	return tsvt.RefOperand{Ref: tsvt.RangeRef{From: ax.set(rangeRef.From, lo), To: &to}}
}

// refError is the #REF! literal a deleted reference collapses to.
func refError() tsvt.Expr { return tsvt.ErrorLit{Code: string(ErrRef)} }

// mapRefs returns expr with every reference operand replaced by f(ref) and its
// structure otherwise preserved.
func mapRefs(expr tsvt.Expr, f func(tsvt.Reference) tsvt.Expr) tsvt.Expr {
	switch e := expr.(type) {
	case tsvt.RefOperand:
		return f(e.Ref)
	case tsvt.Unary:
		return tsvt.Unary{Op: e.Op, X: mapRefs(e.X, f)}
	case tsvt.Percent:
		return tsvt.Percent{X: mapRefs(e.X, f)}
	case tsvt.Binary:
		return tsvt.Binary{Op: e.Op, Left: mapRefs(e.Left, f), Right: mapRefs(e.Right, f)}
	case tsvt.Call:
		return tsvt.Call{Name: e.Name, Args: mapArgs(e.Args, f)}
	default:
		return expr
	}
}

// mapArgs applies mapRefs to each argument of a call.
func mapArgs(args []tsvt.Expr, f func(tsvt.Reference) tsvt.Expr) []tsvt.Expr {
	out := make([]tsvt.Expr, len(args))
	for i, arg := range args {
		out[i] = mapRefs(arg, f)
	}
	return out
}
