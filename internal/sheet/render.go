package sheet

import (
	"strconv"
	"strings"

	"github.com/uplang/tsvsheet.go/internal/tsvt"
)

// RenderExpr reconstructs a readable source form of an expression, used by
// diagnostics and the explain trace.
func RenderExpr(expr tsvt.Expr) string {
	switch e := expr.(type) {
	case tsvt.Number:
		return e.Text
	case tsvt.StringLit:
		return `"` + e.Value + `"`
	case tsvt.BoolLit:
		return renderBool(boolResult(e.IsTrue))
	case tsvt.ErrorLit:
		return e.Code
	case tsvt.RefOperand:
		return RenderReference(e.Ref)
	case tsvt.Unary:
		return string(e.Op) + RenderExpr(e.X)
	case tsvt.Percent:
		return RenderExpr(e.X) + "%"
	case tsvt.Binary:
		return RenderExpr(e.Left) + " " + string(e.Op) + " " + RenderExpr(e.Right)
	default: // tsvt.Call
		return renderCall(expr.(tsvt.Call))
	}
}

// renderBool reconstructs a boolean literal.
func renderBool(isTrue boolResult) string {
	if isTrue {
		return "TRUE"
	}
	return "FALSE"
}

// renderCall reconstructs a function call.
func renderCall(call tsvt.Call) string {
	args := make([]string, len(call.Args))
	for i, arg := range call.Args {
		args[i] = RenderExpr(arg)
	}
	return call.Name + "(" + strings.Join(args, ",") + ")"
}

// RenderReference reconstructs an A1 reference: a cell or a two-cell range.
func RenderReference(ref tsvt.Reference) string {
	rangeRef := ref.(tsvt.RangeRef)
	if rangeRef.To == nil {
		return renderCell(rangeRef.From)
	}
	return renderCell(rangeRef.From) + ":" + renderCell(*rangeRef.To)
}

// renderCell reconstructs one A1 cell (`B2`).
func renderCell(cell tsvt.CellRef) string {
	return cell.Col + strconv.Itoa(cell.Row)
}
