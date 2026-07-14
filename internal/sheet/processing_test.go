package sheet_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/sheet"
	"github.com/uplang/tsvsheet.go/internal/tsvt"
)

func TestProcessing_NoSectionMarkers(t *testing.T) {
	t.Parallel()

	// With no markers the whole template is body (§4 minimal form).
	out := computeGrid(t, "E=A + B", fixedData)
	assert.Equal(t, "3", out[0][4]) // 1+2
	assert.Equal(t, "5", out[1][4]) // 2+3
}

func TestProcessing_PositionalCells(t *testing.T) {
	t.Parallel()

	// A leading row anchor writes nothing; positional formulas map to their
	// field index (ADR 0003 rule 17): field 1 → column B, field 2 → column C.
	// The literal comes first in `10 + A` so `+` is addition, not a row offset
	// (ADR 0003 rule 19).
	out := computeGrid(t, "=body\n*\t=10 + A\t=20 + A", fixedData)
	assert.Equal(t, "11", out[0][1]) // field 1 → column B (index 1)
	assert.Equal(t, "21", out[0][2]) // field 2 → column C (index 2)
}

func TestProcessing_PositionalLiteral(t *testing.T) {
	t.Parallel()

	// A positional literal is placed verbatim at its field index. "Tag" is a
	// mixed-case bareword (an all-caps token would lex as a column reference).
	out := computeGrid(t, "=body\n*\tTag", fixedData)
	assert.Equal(t, "Tag", out[0][1])
}

func TestProcessing_HeaderLiteralAndNamedBinding(t *testing.T) {
	t.Parallel()

	// Header cells bind names; a named reference then resolves. The literal is
	// first in `1 + "Total"` so `+` is addition (ADR 0003 rule 19).
	out := computeGrid(t, "=header(1)\nA\tB\t\"Total\"\tD\n=body\nZ=1 + \"Total\"", fixedData)
	assert.Equal(t, "5", out[1][25]) // Total = column C = 4; 1 + 4 = 5
}

func TestProcessing_FinalPlacement(t *testing.T) {
	t.Parallel()

	// A final placement appends a labeled row and a one-shot aggregate.
	out := computeGrid(t, "=final\nA$+1=Total\nB$=sum(B$1:B$3)", fixedData)
	require.Len(t, out, 4)
	assert.Equal(t, "Total", out[3][0])
	assert.Equal(t, "9", out[3][1]) // sum of B rows 1-3 = 2+3+4
}

func TestProcessing_FinalFormulaAtLastRow(t *testing.T) {
	t.Parallel()

	// A$ targets column A at the last row (row 2).
	out := computeGrid(t, "=final\nA$=sum(A$1:A$3)", fixedData)
	assert.Equal(t, "6", out[2][0]) // overwrites A row 2 with 1+2+3
}

func TestProcessing_PositionalInFinalHasNoRow(t *testing.T) {
	t.Parallel()

	// A positional (non-addressed) cell in the final phase has no current row
	// and writes nothing; the grid is unchanged.
	out := computeGrid(t, "=final\n=A + 1", fixedData)
	assert.Equal(t, sheet.Grid{{"1", "2", "3", "4"}, {"2", "3", "4", "5"}, {"3", "4", "5", "6"}}, out)
}

func TestProcessing_PlacementWithNoPayload(t *testing.T) {
	t.Parallel()

	// A bare reference placement (header label style) with no payload writes
	// nothing.
	out := computeGrid(t, "=body\nE", fixedData)
	assert.Equal(t, fixedData3x4(), out)
}

func TestProcessing_AppendViaAbsoluteRow(t *testing.T) {
	t.Parallel()

	// A$4 addresses one past the 3-row grid → appends row 4. "New" is a
	// mixed-case bareword literal (an all-caps token would lex as a column
	// reference).
	out := computeGrid(t, "=final\nA$4=New", fixedData)
	require.Len(t, out, 4)
	assert.Equal(t, "New", out[3][0])
}

func TestStructural_InsertBefore(t *testing.T) {
	t.Parallel()

	// =A< inserts an empty column before A; the data shifts right.
	out := computeGrid(t, "=final\n=A<", fixedData)
	assert.Equal(t, []string{"", "1", "2", "3", "4"}, out[0])
}

func TestStructural_InsertAfter(t *testing.T) {
	t.Parallel()

	// =A> inserts an empty column after A.
	out := computeGrid(t, "=final\n=A>", fixedData)
	assert.Equal(t, []string{"1", "", "2", "3", "4"}, out[0])
}

func TestStructural_Delete(t *testing.T) {
	t.Parallel()

	// =B! deletes column B; C,D shift left.
	out := computeGrid(t, "=final\n=B!", fixedData)
	assert.Equal(t, []string{"1", "3", "4"}, out[0])
}

func TestStructural_DeleteShiftsNamedColumns(t *testing.T) {
	t.Parallel()

	// Deleting column A shifts the "Val" binding from index 2 to 1; a later
	// final line reading "Val" at an absolute row resolves to the shifted
	// column. After delete, data row 2 = [4,5,6], so Val (index 1) = 5.
	out := computeGrid(t, "=header(1)\nA\tB\tVal\tD\n=final\n=A!\nZ$=\"Val\"$3", fixedData)
	assert.Equal(t, "5", out[2][25])
}

func TestStructural_InsertBeforeShiftsNamedColumns(t *testing.T) {
	t.Parallel()

	// Inserting before A shifts "Val" from index 2 to 3; after insert, data
	// row 2 = ["",3,4,5,6], so Val (index 3) = 5.
	out := computeGrid(t, "=header(1)\nA\tB\tVal\tD\n=final\n=A<\nZ$=\"Val\"$3", fixedData)
	assert.Equal(t, "5", out[2][25])
}

// fixedData3x4 is fixedData as a Grid literal for equality assertions.
func fixedData3x4() sheet.Grid {
	return sheet.Grid{{"1", "2", "3", "4"}, {"2", "3", "4", "5"}, {"3", "4", "5", "6"}}
}

func TestCompute_RejectsFatalStructural(t *testing.T) {
	t.Parallel()

	// A range-scoped structural command is rejected (rule 7).
	tmpl, err := tsvt.Parse(tsvt.Source("=final\n=A:C<"))
	require.NoError(t, err)
	_, err = sheet.Compute(tmpl, fixedData3x4())
	require.Error(t, err)
}
