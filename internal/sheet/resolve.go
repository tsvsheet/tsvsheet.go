package sheet

import (
	"time"

	"github.com/uplang/tsvsheet.go/internal/tsvt"
)

// cellPhase tracks a cell's evaluation state for memoization and cycle
// detection.
type cellPhase int

const (
	phaseUnvisited cellPhase = iota
	phaseVisiting            // on the current evaluation stack → a cycle
	phaseDone
)

// computer memoizes cell values as they are evaluated in dependency order. Its
// cache and phase slices are allocated once and shared, so value-receiver
// methods mutate them in place (no reassignment) and every recursive read sees
// the same state. now is the wall clock sampled once for the pass (volatile
// functions).
type computer struct {
	env    embedEnv
	now    time.Time
	sheet  Sheet
	cache  [][]Value
	phase  [][]cellPhase
	limits Limits
}

// newComputer builds a computer sized to the sheet, with the pass clock and the
// engine's generous DefaultLimits (the plain Compute/ComputeAt path); the
// embedding path (ComputeWith) overrides them with the injected limits.
func newComputer(s Sheet, now time.Time) computer {
	cache := make([][]Value, len(s.cells))
	phase := make([][]cellPhase, len(s.cells))
	for r, row := range s.cells {
		cache[r] = make([]Value, len(row))
		phase[r] = make([]cellPhase, len(row))
	}
	return computer{now: now, sheet: s, cache: cache, phase: phase, limits: DefaultLimits()}
}

// cellValue is a cell's evaluated Value: a literal parsed, a formula computed
// (which may be a dynamic array that later spills).
func (c computer) cellValue(row rowIndex, col colIndex, cl cell) Value {
	if !cl.isFormula() {
		return value(textVal(cl.text))
	}
	return c.read(row, col)
}

// read returns the value at (row, col), evaluating and memoizing it on first
// visit. A cell already on the evaluation stack is a circular reference; an
// out-of-grid position is #REF!.
func (c computer) read(row rowIndex, col colIndex) Value {
	cl, inGrid := c.sheet.at(row, col)
	if !inGrid {
		return errorValue(ErrRef)
	}
	switch c.phase[row][col] {
	case phaseDone:
		return c.cache[row][col]
	case phaseVisiting:
		return errorValue(ErrCirc)
	}
	c.phase[row][col] = phaseVisiting
	result := c.evalCell(cl)
	c.cache[row][col] = result
	c.phase[row][col] = phaseDone
	return result
}

// evalCell evaluates one cell: a literal parses to its value, a formula
// evaluates its expression (which reads other cells).
func (c computer) evalCell(cl cell) Value {
	if !cl.isFormula() {
		return value(textVal(cl.text))
	}
	return resolver{comp: c}.eval(cl.formula)
}

// resolver evaluates expressions against the computer, resolving A1 references.
type resolver struct {
	comp computer
}

// cellset is a resolved reference: the referenced cells' values, and whether it
// was a single cell (a range is not single).
type cellset struct {
	values   []Value
	isSingle boolResult
}

// scalar reduces a cellset to one value: a single cell is its value; a
// multi-cell range used where a scalar is required is #VALUE!.
func (c cellset) scalar() Value {
	if c.isSingle && len(c.values) == 1 {
		return c.values[0]
	}
	return errorValue(ErrValue)
}

// resolveOperand resolves a reference operand: a single A1 cell or an A1 range,
// read from another sheet when the reference carries a `"file"!` qualifier.
func (r resolver) resolveOperand(ref tsvt.Reference) cellset {
	rangeRef := ref.(tsvt.RangeRef)
	if rangeRef.File != "" {
		return r.foreignCells(rangeRef)
	}
	if rangeRef.To == nil {
		return r.resolveSingle(rangeRef.From)
	}
	return r.resolveMatrix(rangeRef.From, *rangeRef.To)
}

// resolveSingle resolves a single-cell reference; an out-of-grid row (`A0`) is
// #REF!, kept isSingle so it propagates through scalar().
func (r resolver) resolveSingle(cell tsvt.CellRef) cellset {
	at, ok := a1Address(cell)
	if !ok {
		return cellset{values: []Value{errorValue(ErrRef)}, isSingle: true}
	}
	return cellset{values: []Value{r.comp.read(rowIndex(at.Row), colIndex(at.Col))}, isSingle: true}
}

// resolveMatrix resolves the rectangular hull of two A1 corners (`A1:B3`).
func (r resolver) resolveMatrix(from, to tsvt.CellRef) cellset {
	a, aok := a1Address(from)
	b, bok := a1Address(to)
	if !aok || !bok {
		return cellset{values: []Value{errorValue(ErrRef)}}
	}
	return cellset{values: r.hull(a, b)}
}

// hull reads every cell in the inclusive rectangle spanned by a and b.
func (r resolver) hull(a, b Address) []Value {
	r0, r1 := ordered(gridPos(a.Row), gridPos(b.Row))
	c0, c1 := ordered(gridPos(a.Col), gridPos(b.Col))
	values := make([]Value, 0, int(r1-r0+1)*int(c1-c0+1))
	for row := r0; row <= r1; row++ {
		for col := c0; col <= c1; col++ {
			values = append(values, r.comp.read(rowIndex(row), colIndex(col)))
		}
	}
	return values
}

// a1Address converts an A1 cell to a 0-based (row, col). ok is false for a row
// below 1 (`A0`); the grammar guarantees a column label and an integer row.
func a1Address(cell tsvt.CellRef) (Address, boolResult) {
	if cell.Row < 1 {
		return Address{}, false
	}
	return Address{Row: cell.Row - 1, Col: lettersToIndex(columnLetters(cell.Col))}, true
}

// ordered returns its two coordinates low-first.
func ordered(x, y gridPos) (gridPos, gridPos) {
	if x <= y {
		return x, y
	}
	return y, x
}

// argMatrix resolves an argument to a 2-D block of values: a range keeps its
// rows×columns shape (for lookups), any other expression is a 1×1 block.
func (r resolver) argMatrix(arg tsvt.Expr) [][]Value {
	if ref, ok := arg.(tsvt.RefOperand); ok {
		return r.rangeMatrix(ref.Ref)
	}
	return [][]Value{{r.eval(arg)}}
}

// rangeMatrix resolves an A1 reference to its rows×columns of values; an
// off-grid endpoint yields a 1×1 #REF! block. A `"file"!` qualifier reads the
// block from another sheet.
func (r resolver) rangeMatrix(ref tsvt.Reference) [][]Value {
	rangeRef := ref.(tsvt.RangeRef)
	if rangeRef.File != "" {
		return r.foreignMatrix(rangeRef)
	}
	from, fromOK := a1Address(rangeRef.From)
	to, toOK := from, fromOK
	if rangeRef.To != nil {
		to, toOK = a1Address(*rangeRef.To)
	}
	if !fromOK || !toOK {
		return [][]Value{{errorValue(ErrRef)}}
	}
	r0, r1 := ordered(gridPos(from.Row), gridPos(to.Row))
	c0, c1 := ordered(gridPos(from.Col), gridPos(to.Col))
	rows := make([][]Value, 0, r1-r0+1)
	for row := r0; row <= r1; row++ {
		cols := make([]Value, 0, c1-c0+1)
		for col := c0; col <= c1; col++ {
			cols = append(cols, r.comp.read(rowIndex(row), colIndex(col)))
		}
		rows = append(rows, cols)
	}
	return rows
}
