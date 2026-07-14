package sheet

import (
	"github.com/uplang/tsvsheet.go/internal/constants"
	"github.com/uplang/tsvsheet.go/internal/tsvt"
)

// Trace explains how one computed cell was produced: its value, the source
// formula that wrote it (empty for a plain data cell), and the resolved value
// of each reference the formula reads.
type Trace struct {
	Cell    Address
	Value   string
	Formula string
	Inputs  []TraceInput
}

// TraceInput is one reference a formula reads, with its resolved value.
type TraceInput struct {
	Ref   string
	Value string
}

// Explain computes the grid and describes the cell at at: its value, and — when
// a body/final formula produces it — that formula and its inputs. The
// producing-formula lookup resolves columns against the final grid, so it is
// exact for templates without structural commands (which shift columns mid
// pass); with structural commands it still reports the value.
func Explain(t tsvt.Template, g Grid, at Address) (Trace, error) {
	out, err := Compute(t, g)
	if err != nil {
		return Trace{}, err
	}
	raw, ok := out.at(at.Row, at.Col)
	if !ok {
		return Trace{}, constants.ErrNotFound.With(nil, "cell", at.String())
	}
	return newTrace(t, g, out, at, raw), nil
}

// newTrace assembles the trace for an in-grid cell, attaching a producing
// formula and its inputs when one is found.
func newTrace(t tsvt.Template, g, out Grid, at Address, raw string) Trace {
	trace := Trace{Cell: at, Value: raw}
	phases := partition(t)
	comp := &computation{grid: out, names: map[string]int{}, width: out.cols()}
	comp.bindHeaders(phases.header)
	expr, evalRow, found := comp.producing(phases, at, g.rows())
	if !found {
		return trace
	}
	trace.Formula = renderExpr(expr)
	trace.Inputs = comp.traceInputs(expr, evalRow)
	return trace
}

// producing finds the formula that last wrote the target cell: a body formula
// for a data row, or a final placement at the exact cell. dataRows is the
// original data-row count (body rows).
func (c *computation) producing(p phase, at Address, dataRows int) (tsvt.Expr, int, bool) {
	var (
		found tsvt.Expr
		row   int
		ok    bool
	)
	if at.Row < dataRows {
		found, row, ok = c.scanBody(p.body, at)
	}
	if e, r, hit := c.scanFinal(p.final, at); hit {
		found, row, ok = e, r, true
	}
	return found, row, ok
}

// scanBody returns the last body formula whose column matches at, evaluated at
// at.Row.
func (c *computation) scanBody(lines []tsvt.Line, at Address) (tsvt.Expr, int, bool) {
	var (
		found tsvt.Expr
		ok    bool
	)
	for _, line := range lines {
		if e, hit := c.lineFormula(line, at.Col, at.Row, at.Row); hit {
			found, ok = e, true
		}
	}
	return found, at.Row, ok
}

// scanFinal returns the last final formula whose exact cell matches at.
func (c *computation) scanFinal(lines []tsvt.Line, at Address) (tsvt.Expr, int, bool) {
	var (
		found tsvt.Expr
		ok    bool
	)
	for _, line := range lines {
		if e, hit := c.lineFinalFormula(line, at); hit {
			found, ok = e, true
		}
	}
	return found, at.Row, ok
}

// asRow narrows a template line to a cell Row; a section/structural line is not
// a Row.
func asRow(line tsvt.Line) (tsvt.Row, bool) {
	row, ok := line.(tsvt.Row)
	return row, ok
}

// lineFormula returns a body row's formula targeting column col at dataRow.
func (c *computation) lineFormula(line tsvt.Line, col, dataRow, evalRow int) (tsvt.Expr, bool) {
	row, ok := asRow(line)
	if !ok {
		return nil, false
	}
	for field, cell := range row.Cells {
		if expr, hit := c.cellFormula(cell); hit && c.cellCol(cell, field, dataRow) == col {
			return expr, true
		}
	}
	return nil, false
}

// lineFinalFormula returns a final row's placement formula targeting the exact
// address at.
func (c *computation) lineFinalFormula(line tsvt.Line, at Address) (tsvt.Expr, bool) {
	row, ok := asRow(line)
	if !ok {
		return nil, false
	}
	for _, cell := range row.Cells {
		if expr, hit := c.finalPlacementFormula(cell, at); hit {
			return expr, true
		}
	}
	return nil, false
}

// finalPlacementFormula returns a placement's formula when it targets the exact
// address at (final phase, no current row).
func (c *computation) finalPlacementFormula(cell tsvt.Cell, at Address) (tsvt.Expr, bool) {
	placement, ok := cell.(tsvt.PlacementCell)
	if !ok {
		return nil, false
	}
	formula, ok := placement.Payload.(tsvt.FormulaPayload)
	if !ok {
		return nil, false
	}
	row, col, ok := c.target(placement.Ref, noRow)
	if !ok || row != at.Row || col != at.Col {
		return nil, false
	}
	return formula.Expr, true
}

// cellFormula returns a cell's formula expression, if it has one.
func (c *computation) cellFormula(cell tsvt.Cell) (tsvt.Expr, bool) {
	switch cl := cell.(type) {
	case tsvt.FormulaCell:
		return cl.Expr, true
	case tsvt.PlacementCell:
		if formula, ok := cl.Payload.(tsvt.FormulaPayload); ok {
			return formula.Expr, true
		}
	}
	return nil, false
}

// cellCol returns the column a cell targets: a positional cell its field index,
// a placement its resolved column (-1 when unresolvable).
func (c *computation) cellCol(cell tsvt.Cell, field, dataRow int) int {
	placement, ok := cell.(tsvt.PlacementCell)
	if !ok {
		return field
	}
	_, col, ok := c.target(placement.Ref, dataRow)
	if !ok {
		return -1
	}
	return col
}

// traceInputs renders each reference operand in expr with its resolved value at
// the given row.
func (c *computation) traceInputs(expr tsvt.Expr, evalRow int) []TraceInput {
	res := c.resolverAt(evalRow)
	var inputs []TraceInput
	walkRefs(expr, func(ref tsvt.Reference) {
		inputs = append(inputs, TraceInput{Ref: renderRef(ref), Value: res.resolveOperand(ref).scalar().String()})
	})
	return inputs
}

// walkRefs visits every reference operand in an expression tree.
func walkRefs(expr tsvt.Expr, visit func(tsvt.Reference)) {
	switch e := expr.(type) {
	case tsvt.RefOperand:
		visit(e.Ref)
	case tsvt.Unary:
		walkRefs(e.X, visit)
	case tsvt.Binary:
		walkRefs(e.Left, visit)
		walkRefs(e.Right, visit)
	case tsvt.Call:
		for _, arg := range e.Args {
			walkRefs(arg, visit)
		}
	}
}
