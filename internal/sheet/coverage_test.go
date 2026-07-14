package sheet_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/sheet"
)

func TestArithmetic_LeftAndRightStringErrors(t *testing.T) {
	t.Parallel()

	// `*` is not a row modifier, so both operands are strings via unbound named
	// columns; each side's #VALUE! is exercised.
	assert.Equal(t, string(sheet.ErrValue), eval1(t, `"apple" * 2`)) // left
	assert.Equal(t, string(sheet.ErrValue), eval1(t, `2 * "apple"`)) // right
}

func TestAggregate_ErrorShortCircuits(t *testing.T) {
	t.Parallel()

	const data = "#REF!\t2\n"
	// min/max/avg short-circuit on an error operand (numerics ok=false).
	assert.Equal(t, string(sheet.ErrRef), evalData(t, "min(A:B)", data))
	assert.Equal(t, string(sheet.ErrRef), evalData(t, "max(A:B)", data))
	assert.Equal(t, string(sheet.ErrRef), evalData(t, "avg(A:B)", data))
}

func TestTarget_UnresolvableSkips(t *testing.T) {
	t.Parallel()

	// A placement whose target is a range (not a single cell) writes nothing.
	out := computeGrid(t, "=body\nA:C=99", fixedData)
	assert.Equal(t, fixedData3x4(), out)
}

func TestTarget_ElidedColumnSkips(t *testing.T) {
	t.Parallel()

	// A placement onto an elided-column numeric target has no column → skip.
	out := computeGrid(t, "=body\n[,0]=99", fixedData)
	assert.Equal(t, fixedData3x4(), out)
}

func TestTarget_FinalBareReferenceSkips(t *testing.T) {
	t.Parallel()

	// In the final phase a placement whose row is elided (current row) has no
	// current row → skip.
	out := computeGrid(t, "=final\nE=99", fixedData)
	assert.Equal(t, fixedData3x4(), out)
}

func TestMatrix_NonCellEndpoint(t *testing.T) {
	t.Parallel()

	// A matrix endpoint that is a row selector (not a cell) is #REF! (rule 4).
	assert.Equal(t, string(sheet.ErrRef), eval1(t, "sum(*1:C1)"))
}

func TestGroupedRange_UnboundNamed(t *testing.T) {
	t.Parallel()

	// A grouped range over unbound named columns cannot resolve → #REF!.
	assert.Equal(t, string(sheet.ErrRef), eval1(t, `sum(("X":"Y")0)`))
}

func TestRowSelector_WidthFromData(t *testing.T) {
	t.Parallel()

	// A whole-row sum spans the logical data width even after computing a far
	// column: sum(*) at row 1 = A+B+C+D = 14, unaffected by column Z.
	assert.Equal(t, "14", eval1(t, "sum(*)"))
}

func TestStructural_UnboundNamedColumnSkips(t *testing.T) {
	t.Parallel()

	// A structural command on an unbound named column cannot resolve → no-op.
	out := computeGrid(t, "=final\n=\"Missing\"<", fixedData)
	assert.Equal(t, fixedData3x4(), out)
}

func TestStructural_InsertPastRaggedRow(t *testing.T) {
	t.Parallel()

	// Inserting after a column past a short row appends at that row's end.
	out := computeGrid(t, "=final\n=C>", "1\t2\t3\t4\n5\n")
	assert.Equal(t, []string{"1", "2", "3", "", "4"}, out[0]) // full row: insert after C
	assert.Equal(t, []string{"5", ""}, out[1])                // short row: empty appended at its end
}

func TestValue_TextDataCell(t *testing.T) {
	t.Parallel()

	// A plain text data cell round-trips as a string (not a number or error).
	assert.Equal(t, "hello", evalData(t, "A", "hello\t2\n"))
	assert.Equal(t, "hello", eval1viaData(t, `A`, "hello\t2\n"))
}

// eval1viaData is evalData at row 0 (alias for readability in comparisons).
func eval1viaData(t *testing.T, formula, data string) string {
	t.Helper()
	return evalData(t, formula, data)
}

func TestValue_EmptyComparedToString(t *testing.T) {
	t.Parallel()

	// Comparing an empty cell to a string exercises the empty-text path.
	assert.Equal(t, "0", evalData(t, `if(B = "x", 1, 0)`, "1\t\t3\n"))
}

func TestRagged_MissingTrailingCell(t *testing.T) {
	t.Parallel()

	// B$2 addresses a cell in a short row (present row, missing column) → empty.
	assert.Equal(t, "", evalData(t, "B$2", "1\t2\n3\n"))
}

func TestHeader_FormulaAndRangeCellsBindNothing(t *testing.T) {
	t.Parallel()

	// A header line with a formula cell and a range cell binds no names; a
	// bareword cell still binds. The named reference resolves to the bareword's
	// column.
	out := computeGrid(t, "=header(1)\n=C + D\tName\tA:B\tD\n=body\nZ=\"Name\"", fixedData)
	assert.Equal(t, "3", out[1][25]) // "Name" bound to column B (index 1) = 3 at row 1
}

func TestExplain_UnaryAndNumberAndLiteral(t *testing.T) {
	t.Parallel()

	// A unary formula with a numeric literal exercises renderExpr's unary and
	// number branches and walkRefs' unary branch. A preceding literal cell (no
	// formula) exercises cellFormula's non-formula branch.
	trace := explain(t, "=body\nY=Tag\tZ=0 - C", fixedData, "Z2")
	assert.Equal(t, "0 - C", trace.Formula)
	require.Len(t, trace.Inputs, 1)
	assert.Equal(t, "C", trace.Inputs[0].Ref)
}

func TestExplain_UnaryOperand(t *testing.T) {
	t.Parallel()

	// -C exercises the unary render/walk path directly.
	trace := explain(t, "=body\nZ=0 - abs(C)", fixedData, "Z2")
	assert.Contains(t, trace.Formula, "abs(C)")
}

func TestExplain_FinalLiteralPayloadAndMismatch(t *testing.T) {
	t.Parallel()

	// A positional formula cell (not a placement) and a literal-payload
	// placement (not a formula) exercise finalPlacementFormula's non-placement
	// and non-formula branches; an absolute-row formula placement is matched
	// (an absolute row is stable across the appended row, unlike `$`).
	trace := explain(t, "=final\n=99 + A\nA$+1=Tag\nC$3=sum(C$1:C$3)", fixedData, "C3")
	assert.Equal(t, "sum(C$1:C$3)", trace.Formula)
}

func TestExplain_BodyWithStructural(t *testing.T) {
	t.Parallel()

	// A structural command in the body section is skipped by the body formula
	// lookup (asRow false); the addressed formula still produces the cell.
	trace := explain(t, "=body\n=$>\nZ=A", fixedData, "Z1")
	assert.Equal(t, "A", trace.Formula)
}

func TestExplain_CellColUnresolvable(t *testing.T) {
	t.Parallel()

	// A body placement whose target column is an unbound named column resolves
	// to no column (-1) in the producing-formula scan; explaining a different
	// cell still succeeds.
	trace := explain(t, "=body\n\"Missing\"=C\nZ=D", fixedData, "Z1")
	assert.Equal(t, "D", trace.Formula)
}

func TestExplain_PositionalFormulaProducer(t *testing.T) {
	t.Parallel()

	// A positional formula at field 0 targets column A; explaining A1 finds it
	// (exercises cellCol's positional branch and cellFormula on a formula cell).
	trace := explain(t, "=body\n=D", fixedData, "A1")
	assert.Equal(t, "D", trace.Formula)
}

func TestExplain_LiteralCellSkippedInScan(t *testing.T) {
	t.Parallel()

	// A positional literal cell has no formula; the scan skips it (cellFormula
	// default branch) and finds the addressed formula.
	trace := explain(t, "=body\nTag\tZ=A", fixedData, "Z1")
	assert.Equal(t, "A", trace.Formula)
}

func TestExplain_UnaryRenderAndWalk(t *testing.T) {
	t.Parallel()

	// A unary formula exercises renderExpr's unary branch and walkRefs' unary
	// branch; the numeric operand exercises walkRefs' no-op and renderExpr's
	// number branch.
	trace := explain(t, "=body\nZ=-C\nY=1 + C", fixedData, "Z2")
	assert.Equal(t, "-C", trace.Formula)
	require.Len(t, trace.Inputs, 1)
	assert.Equal(t, "C", trace.Inputs[0].Ref)
}

func TestExplain_NumericGroupedRender(t *testing.T) {
	t.Parallel()

	// A numeric grouped range renders its column indices (renderCol's index
	// branch).
	trace := explain(t, "=body\nZ=sum(([0]:[2])0)", fixedData, "Z2")
	require.Len(t, trace.Inputs, 1)
	assert.Equal(t, "([0]:[2])0", trace.Inputs[0].Ref)
}

func TestTargetRow_NegativeSkips(t *testing.T) {
	t.Parallel()

	// A body placement to a row before the grid (A1 at row 0 → row -1) skips.
	// Single-row data so no later row writes back into row 0.
	out := computeGrid(t, "=body\nA1=99", "1\t2\t3\t4\n")
	assert.Equal(t, sheet.Grid{{"1", "2", "3", "4"}}, out)
}

func TestMatrix_UnresolvableCorner(t *testing.T) {
	t.Parallel()

	// A matrix endpoint on an unbound named column cannot resolve → #REF!.
	assert.Equal(t, string(sheet.ErrRef), eval1(t, `sum("X"$1:C$3)`))
}

func TestRowSelector_FinalPhaseNoRow(t *testing.T) {
	t.Parallel()

	// A whole-row selector in the final phase has no current row → #REF!.
	out := computeGrid(t, "=final\nA$3=sum(*)", fixedData)
	assert.Equal(t, string(sheet.ErrRef), out[2][0])
}

func TestStructural_DeleteDropsNamedColumn(t *testing.T) {
	t.Parallel()

	// Deleting the exact column a name binds drops that binding; a later
	// reference to the (now unbound) name becomes a string literal.
	out := computeGrid(t, "=header(1)\nA\tB\tVal\tD\n=final\n=C!\nZ$3=\"Val\"", fixedData)
	assert.Equal(t, "Val", out[2][25]) // Val unbound after delete → string literal
}

func TestExplain_FinalFormulaMismatch(t *testing.T) {
	t.Parallel()

	// Two absolute-row formula placements; explaining C3 skips the A-column
	// placement (target mismatch) and matches the C-column one.
	trace := explain(t, "=final\nA$3=sum(A$1:A$3)\nC$3=sum(C$1:C$3)", fixedData, "C3")
	assert.Equal(t, "sum(C$1:C$3)", trace.Formula)
}

func TestHeader_RowSelectorBindsNothing(t *testing.T) {
	t.Parallel()

	// A row-selector header cell (`*`) is not a named column; it binds nothing.
	// The bareword "Name" still binds its column.
	out := computeGrid(t, "=header(1)\n*\tName\tC\tD\n=body\nZ=\"Name\"", fixedData)
	assert.Equal(t, "3", out[1][25]) // Name → column B = 3 at row 1
}
