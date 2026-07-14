package sheet

import (
	"strconv"

	"github.com/uplang/tsvsheet.go/internal/tsvt"
)

// LineKind names a template line's shape for structural output (the parse
// command).
type LineKind string

// The line kinds.
const (
	KindHeader     LineKind = "header"
	KindBody       LineKind = "body"
	KindFinal      LineKind = "final"
	KindStructural LineKind = "structural"
	KindRow        LineKind = "row"
)

// LineKindOf reports a template line's kind.
func LineKindOf(line tsvt.Line) LineKind {
	switch line.(type) {
	case tsvt.HeaderMarker:
		return KindHeader
	case tsvt.BodyMarker:
		return KindBody
	case tsvt.FinalMarker:
		return KindFinal
	case tsvt.Structural:
		return KindStructural
	default: // tsvt.Row
		return KindRow
	}
}

// RenderLine reconstructs a template line's source form.
func RenderLine(line tsvt.Line) string {
	switch l := line.(type) {
	case tsvt.HeaderMarker:
		return "=header(" + strconv.Itoa(l.Count) + ")"
	case tsvt.BodyMarker:
		return "=body"
	case tsvt.FinalMarker:
		return "=final"
	case tsvt.Structural:
		return "=" + RenderReference(l.Ref) + string(l.Mod)
	default: // tsvt.Row
		return renderRowSource(l.(tsvt.Row))
	}
}

// renderRowSource joins a row's cells with TABs.
func renderRowSource(row tsvt.Row) string {
	cells := make([]string, len(row.Cells))
	for i, cell := range row.Cells {
		cells[i] = RenderCell(cell)
	}
	return join(cells)
}

// join concatenates cell sources with a TAB separator.
func join(cells []string) string {
	out := ""
	for i, c := range cells {
		if i > 0 {
			out += tab
		}
		out += c
	}
	return out
}

// RenderCell reconstructs a cell's source form.
func RenderCell(cell tsvt.Cell) string {
	switch c := cell.(type) {
	case tsvt.FormulaCell:
		return "=" + RenderExpr(c.Expr)
	case tsvt.LiteralCell:
		return c.Value.Text
	case tsvt.PlacementCell:
		return RenderReference(c.Ref) + string(c.Mod) + renderPayload(c.Payload)
	default: // tsvt.EmptyCell
		return ""
	}
}

// renderPayload reconstructs a placement payload's source, including the `=`
// separator; a nil payload renders empty.
func renderPayload(payload tsvt.Payload) string {
	switch p := payload.(type) {
	case tsvt.FormulaPayload:
		return "=" + RenderExpr(p.Expr)
	case tsvt.LiteralPayload:
		return "=" + p.Value.Text
	default: // nil
		return ""
	}
}
