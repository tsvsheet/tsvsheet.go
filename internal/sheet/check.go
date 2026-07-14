package sheet

import (
	"strings"

	"github.com/uplang/tsvsheet.go/internal/constants"
	"github.com/uplang/tsvsheet.go/internal/tsvt"
)

// Diagnostic is a static template finding. A Fatal diagnostic makes Compute
// reject the template; a non-fatal one is advisory (the construct still
// computes, e.g. an unknown function to #NAME?).
type Diagnostic struct {
	Line    tsvt.LineNumber
	Message string
	Fatal   bool
}

// err renders a fatal diagnostic as its sentinel error.
func (d Diagnostic) err() error {
	return constants.ErrUnsupported.With(nil, "line", int(d.Line), "message", d.Message)
}

// Check returns the static diagnostics of a template: rejected structural forms
// (ADR 0003 rules 7, 18) as fatal, unknown function names (rule 10) as
// advisory. It needs no data grid.
func Check(t tsvt.Template) []Diagnostic {
	var diags []Diagnostic
	for _, line := range t.Lines {
		diags = append(diags, checkLine(line)...)
	}
	return diags
}

// checkLine collects the diagnostics of one template line.
func checkLine(line tsvt.Line) []Diagnostic {
	switch l := line.(type) {
	case tsvt.Structural:
		return checkStructural(l)
	case tsvt.Row:
		return checkRow(l)
	default:
		return nil
	}
}

// checkStructural rejects a structural command whose target is not a single
// bare column: a range-scoped form (rule 7) or a per-cell/row form (rule 18).
func checkStructural(cmd tsvt.Structural) []Diagnostic {
	if !isBareColumn(cmd.Ref) {
		return []Diagnostic{{Line: cmd.At, Message: "structural command requires a single column (§6 range/cell scope is open)", Fatal: true}}
	}
	return nil
}

// isBareColumn reports whether a reference is a single column with no row and no
// range — the only structural target ADR 0003 rule 18 admits.
func isBareColumn(ref tsvt.Reference) bool {
	cell, ok := placementCell(ref)
	return ok && cell.Row == nil
}

// checkRow collects diagnostics for a cell row: a per-cell structural modifier
// (rule 18) is fatal; unknown functions in any formula are advisory.
func checkRow(row tsvt.Row) []Diagnostic {
	var diags []Diagnostic
	for _, cell := range row.Cells {
		diags = append(diags, checkCell(cell, row.At)...)
	}
	return diags
}

// checkCell collects the diagnostics of one cell.
func checkCell(cell tsvt.Cell, at tsvt.LineNumber) []Diagnostic {
	switch c := cell.(type) {
	case tsvt.FormulaCell:
		return unknownFuncs(c.Expr, at)
	case tsvt.PlacementCell:
		return checkPlacement(c, at)
	default:
		return nil
	}
}

// checkPlacement rejects a per-cell structural modifier and checks the payload
// formula for unknown functions.
func checkPlacement(cell tsvt.PlacementCell, at tsvt.LineNumber) []Diagnostic {
	if cell.Mod != tsvt.ModNone {
		return []Diagnostic{{Line: at, Message: "per-cell structural modifier is unsupported (§6 open)", Fatal: true}}
	}
	if formula, ok := cell.Payload.(tsvt.FormulaPayload); ok {
		return unknownFuncs(formula.Expr, at)
	}
	return nil
}

// unknownFuncs walks an expression and reports each call to a name outside the
// builtin set as an advisory diagnostic (rule 10).
func unknownFuncs(expr tsvt.Expr, at tsvt.LineNumber) []Diagnostic {
	var diags []Diagnostic
	walkExpr(expr, func(call tsvt.Call) {
		if !isKnownFunc(call.Name) {
			diags = append(diags, Diagnostic{Line: at, Message: "unknown function: " + call.Name})
		}
	})
	return diags
}

// isKnownFunc reports whether name (case-insensitive) is a builtin, including
// the lazily-dispatched `if`.
func isKnownFunc(name string) bool {
	lower := strings.ToLower(name)
	if lower == "if" {
		return true
	}
	_, ok := functions[lower]
	return ok
}

// walkExpr visits every function call in an expression tree.
func walkExpr(expr tsvt.Expr, visit func(tsvt.Call)) {
	switch e := expr.(type) {
	case tsvt.Unary:
		walkExpr(e.X, visit)
	case tsvt.Binary:
		walkExpr(e.Left, visit)
		walkExpr(e.Right, visit)
	case tsvt.Call:
		visit(e)
		for _, arg := range e.Args {
			walkExpr(arg, visit)
		}
	}
}
