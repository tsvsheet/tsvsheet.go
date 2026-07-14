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
	case tsvt.RefOperand:
		return RenderReference(e.Ref)
	case tsvt.Unary:
		return string(e.Op) + RenderExpr(e.X)
	case tsvt.Binary:
		return RenderExpr(e.Left) + " " + string(e.Op) + " " + RenderExpr(e.Right)
	default: // tsvt.Call
		return renderCall(expr.(tsvt.Call))
	}
}

// renderCall reconstructs a function call.
func renderCall(call tsvt.Call) string {
	args := make([]string, len(call.Args))
	for i, arg := range call.Args {
		args[i] = RenderExpr(arg)
	}
	return call.Name + "(" + strings.Join(args, ",") + ")"
}

// RenderReference reconstructs a reference.
func RenderReference(ref tsvt.Reference) string {
	switch r := ref.(type) {
	case tsvt.RangeRef:
		return renderRange(r)
	default: // tsvt.GroupedRange
		return renderGrouped(ref.(tsvt.GroupedRange))
	}
}

// renderRange reconstructs a single endpoint or a two-endpoint range.
func renderRange(ref tsvt.RangeRef) string {
	if ref.To == nil {
		return renderEndpoint(ref.From)
	}
	return renderEndpoint(ref.From) + ":" + renderEndpoint(ref.To)
}

// renderGrouped reconstructs a grouped column range.
func renderGrouped(ref tsvt.GroupedRange) string {
	return "(" + renderCol(ref.FromCol) + ":" + renderCol(ref.ToCol) + ")" + renderRow(ref.Row)
}

// renderEndpoint reconstructs one range endpoint, using numeric `[col,row]`
// form for numeric columns and letter form otherwise.
func renderEndpoint(ep tsvt.Endpoint) string {
	cell, ok := ep.(tsvt.CellEndpoint)
	if !ok {
		return "*" + renderRow(ep.(tsvt.RowSelector).Row)
	}
	if col, numeric := numericColText(cell.Col); numeric {
		return renderNumeric(col, cell.Row)
	}
	return renderCol(cell.Col) + renderRow(cell.Row)
}

// numericColText returns the bracket-inner column text and true for a numeric
// column (ColIndex or ColElided).
func numericColText(col tsvt.Col) (string, bool) {
	switch c := col.(type) {
	case tsvt.ColIndex:
		return strconv.Itoa(c.Index), true
	case tsvt.ColElided:
		return "", true
	default:
		return "", false
	}
}

// renderNumeric reconstructs a numeric `[col,row]` reference; a nil row omits
// the comma (`[3]`).
func renderNumeric(col string, row tsvt.RowRef) string {
	if row == nil {
		return "[" + col + "]"
	}
	return "[" + col + "," + renderNumericRow(row) + "]"
}

// renderNumericRow reconstructs the row part of a numeric reference, where a
// from-end row is a bare negative index.
func renderNumericRow(row tsvt.RowRef) string {
	if r, ok := row.(tsvt.RowFromEnd); ok {
		return "-" + strconv.Itoa(r.N)
	}
	return renderRow(row)
}

// renderCol reconstructs a column reference. It is only reached for letter,
// last, named, and (via a grouped numeric range) index columns; an elided
// column never arrives here because numericColText intercepts it in
// renderEndpoint.
func renderCol(col tsvt.Col) string {
	switch c := col.(type) {
	case tsvt.ColLetters:
		return absMark(c.Abs) + c.Name
	case tsvt.ColLast:
		return "$"
	case tsvt.ColNamed:
		return `"` + c.Name + `"`
	default: // tsvt.ColIndex (a grouped numeric range column)
		return "[" + strconv.Itoa(col.(tsvt.ColIndex).Index) + "]"
	}
}

// absMark renders the `$` absolute-column prefix.
func absMark(abs bool) string {
	if abs {
		return "$"
	}
	return ""
}

// renderRow reconstructs a row reference; a nil row renders empty.
func renderRow(row tsvt.RowRef) string {
	switch r := row.(type) {
	case tsvt.RowBefore:
		return strconv.Itoa(r.N)
	case tsvt.RowAfter:
		return "+" + strconv.Itoa(r.N)
	case tsvt.RowAll:
		return "*"
	case tsvt.RowLast:
		return "$" + offsetMark(r.Offset)
	case tsvt.RowAbs:
		return "$" + strconv.Itoa(r.N)
	default: // nil (a from-end row appears only in numeric refs, rendered there)
		return ""
	}
}

// offsetMark renders a last-row offset suffix (`+1`, `-1`, or empty for 0).
func offsetMark(offset int) string {
	if offset > 0 {
		return "+" + strconv.Itoa(offset)
	}
	if offset < 0 {
		return strconv.Itoa(offset)
	}
	return ""
}
