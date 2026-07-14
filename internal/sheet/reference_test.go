package sheet_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uplang/tsvsheet.go/internal/sheet"
)

func TestRef_ColumnForms(t *testing.T) {
	t.Parallel()

	// At row 1: A=2 B=3 C=4 D=5.
	cases := map[string]string{
		"A":    "2", // letter
		"D":    "5", // letter
		"$":    "5", // last column (D)
		"[0]":  "2", // numeric index
		"[3]":  "5", // numeric index
		"[-1]": "5", // negative from end
		"[-2]": "4", // negative from end
	}
	for formula, want := range cases {
		t.Run(formula, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, eval1(t, formula))
		})
	}
}

func TestRef_RowForms(t *testing.T) {
	t.Parallel()

	// At row 1 (middle): current row A=2; row 0 A=1; row 2 A=3.
	cases := map[string]string{
		"A0":    "2", // current
		"A":     "2", // current (elided)
		"A1":    "1", // one before (row 0)
		"A+1":   "3", // one after (row 2)
		"A$":    "3", // last row (row 2)
		"A$1":   "1", // absolute row 1
		"A$3":   "3", // absolute row 3
		"A$-1":  "2", // last minus one (row 1)
		"[0,1]": "1", // numeric: col 0, one before
	}
	for formula, want := range cases {
		t.Run(formula, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, eval1(t, formula))
		})
	}
}

func TestRef_NumericFromEnd(t *testing.T) {
	t.Parallel()

	// [0,-1] is column 0, last row (row 2) → A=3.
	assert.Equal(t, "3", eval1(t, "[0,-1]"))
	// [0,-3] is column 0, 3rd from end (row 0) → A=1.
	assert.Equal(t, "1", eval1(t, "[0,-3]"))
}

func TestRef_Named(t *testing.T) {
	t.Parallel()

	// A header binds "Val" to a column; the named reference reads it.
	out := computeGrid(t, "=header(1)\nA\tB\tVal\tD\n=body\nZ=\"Val\"", fixedData)
	assert.Equal(t, "4", out[1][25]) // "Val" is column C = 4 at row 1
}

func TestRef_NamedUnboundIsStringLiteral(t *testing.T) {
	t.Parallel()

	// With no header binding, a quoted token is the string literal of its name
	// (ADR 0003 rule 16).
	assert.Equal(t, "hello", eval1(t, `"hello"`))
	assert.Equal(t, "hellothere", eval1(t, `concat("hello", "there")`))
}

func TestRef_Matrix(t *testing.T) {
	t.Parallel()

	// sum over the matrix A$1:B$3 = (1+2)+(2+3)+(3+4) = 15.
	assert.Equal(t, "15", eval1(t, "sum(A$1:B$3)"))
}

func TestRef_MatrixOutOfGrid(t *testing.T) {
	t.Parallel()

	// A matrix endpoint out of the grid makes the whole matrix #REF! (rule 4).
	assert.Equal(t, string(sheet.ErrRef), eval1(t, "sum(A2:B3)")) // A2 at row1 → row -1
}

func TestRef_GroupedRange(t *testing.T) {
	t.Parallel()

	// (A:C)0 is columns A,B,C at the current row → 2+3+4 = 9 at row 1.
	assert.Equal(t, "9", eval1(t, "sum((A:C)0)"))
}

func TestRef_GroupedRangeOutOfGrid(t *testing.T) {
	t.Parallel()

	// A grouped range whose row is out of grid is #REF!.
	assert.Equal(t, string(sheet.ErrRef), eval1(t, "sum((A:C)2)")) // row -1
}

func TestRef_RowSelector(t *testing.T) {
	t.Parallel()

	// * (whole current row) summed = A+B+C+D at row 1 = 2+3+4+5 = 14.
	assert.Equal(t, "14", eval1(t, "sum(*)"))
}

func TestRef_RowSelectorOutOfGrid(t *testing.T) {
	t.Parallel()

	// *2 (row -1 at row 1) is out of grid → #REF!.
	assert.Equal(t, string(sheet.ErrRef), eval1(t, "sum(*2)"))
}

func TestRef_RangeInScalarContext(t *testing.T) {
	t.Parallel()

	// A range used where a scalar is required is #VALUE! (rule 8).
	assert.Equal(t, string(sheet.ErrValue), eval1(t, "A$1:B$3 + 1"))
}

func TestRef_LastColumnOnEmptyRow(t *testing.T) {
	t.Parallel()

	// $ (last column) on a one-column grid.
	out := computeGrid(t, "=body\nB=$", "7\n")
	assert.Equal(t, "7", out[0][1]) // last column is A=7
}
