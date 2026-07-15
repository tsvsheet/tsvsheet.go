package sheet

import (
	"strings"

	"github.com/uplang/tsvsheet.go/internal/tsvt"
)

// Diagnostic is an advisory finding about a formula cell: currently an unknown
// function call (which computes to #NAME?).
type Diagnostic struct {
	Cell    string `json:"cell"`
	Message string `json:"message"`
	IsFatal bool   `json:"fatal"`
}

// Check reports the static diagnostics of a parsed sheet: each unknown function
// call. Syntax errors are already rejected by Parse, and every reference the
// narrowed grammar admits is a valid A1 form, so Check never reports those.
func Check(s Sheet) []Diagnostic {
	var diags []Diagnostic
	for r, row := range s.cells {
		for c, cl := range row {
			if cl.isFormula() {
				diags = append(diags, unknownFunctions(cl.formula, Address{Row: r, Col: c})...)
			}
		}
	}
	return diags
}

// unknownFunctions flags each call to a name outside the builtin set.
func unknownFunctions(expr tsvt.Expr, at Address) []Diagnostic {
	label := at.String()
	var diags []Diagnostic
	walkCalls(expr, func(call tsvt.Call) {
		if !isKnownFunc(funcName(call.Name)) {
			diags = append(diags, Diagnostic{Cell: label, Message: "unknown function: " + call.Name})
		}
	})
	return diags
}

// isKnownFunc reports whether name (case-insensitive) is a builtin, including
// the lazily-dispatched `if`.
func isKnownFunc(name funcName) boolResult {
	lower := strings.ToLower(string(name))
	if lower == "if" {
		return true
	}
	_, ok := functions[lower]
	return boolResult(ok)
}

// walkCalls visits every function call in an expression tree.
func walkCalls(expr tsvt.Expr, visit func(tsvt.Call)) {
	if call, ok := expr.(tsvt.Call); ok {
		visit(call)
	}
	for _, child := range children(expr) {
		walkCalls(child, visit)
	}
}

// children returns the sub-expressions of an expression (empty for a leaf).
func children(expr tsvt.Expr) []tsvt.Expr {
	switch e := expr.(type) {
	case tsvt.Unary:
		return []tsvt.Expr{e.X}
	case tsvt.Percent:
		return []tsvt.Expr{e.X}
	case tsvt.Binary:
		return []tsvt.Expr{e.Left, e.Right}
	case tsvt.Call:
		return e.Args
	default:
		return nil
	}
}
