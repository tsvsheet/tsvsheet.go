package sheet_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/constants"
	"github.com/uplang/tsvsheet.go/internal/sheet"
)

func TestCompute_StringComparison(t *testing.T) {
	t.Parallel()

	// A1="apple", B1="banana" (text literals); the formula compares them.
	cases := map[string]string{
		"=if(A1 < B1, 1, 0)":  "1",
		"=if(B1 < A1, 1, 0)":  "0",
		"=if(A1 = A1, 1, 0)":  "1",
		"=if(A1 <> B1, 1, 0)": "1",
		"=if(B1 > A1, 1, 0)":  "1",
		"=if(A1 <= A1, 1, 0)": "1",
		"=if(A1 >= A1, 1, 0)": "1",
	}
	for expr, want := range cases {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()
			g := compute(t, "apple\tbanana\t"+expr+"\n")
			assert.Equal(t, want, cellAt(t, g, 0, 2))
		})
	}
}

func TestCompute_MixedComparisonAndArithmetic(t *testing.T) {
	t.Parallel()

	// Comparing/adding a text cell to a number is #VALUE!.
	g := compute(t, "apple\t=A1 < 5\t=1 + A1\t=if(A1, 1, 0)\n")
	assert.Equal(t, string(sheet.ErrValue), cellAt(t, g, 0, 1)) // string < number
	assert.Equal(t, string(sheet.ErrValue), cellAt(t, g, 0, 2)) // 1 + string
	assert.Equal(t, "1", cellAt(t, g, 0, 3))                    // non-empty string is truthy
}

func TestCompute_EmptyCells(t *testing.T) {
	t.Parallel()

	// A1 is empty.
	g := compute(t, "\t=A1\t=1 + A1\t=sum(A1:A1)\t=if(A1, 1, 0)\n")
	assert.Equal(t, "", cellAt(t, g, 0, 1))  // empty renders empty
	assert.Equal(t, "1", cellAt(t, g, 0, 2)) // empty is 0 in arithmetic
	assert.Equal(t, "0", cellAt(t, g, 0, 3)) // empty excluded from sum
	assert.Equal(t, "0", cellAt(t, g, 0, 4)) // empty is falsy
}

func TestCompute_EmptyAggregates(t *testing.T) {
	t.Parallel()

	g := compute(t, "\t\t=min(A1:B1)\t=max(A1:B1)\t=avg(A1:B1)\n") // A1,B1 empty
	assert.Equal(t, string(sheet.ErrValue), cellAt(t, g, 0, 2))    // min of nothing
	assert.Equal(t, string(sheet.ErrValue), cellAt(t, g, 0, 3))    // max of nothing
	assert.Equal(t, string(sheet.ErrDiv), cellAt(t, g, 0, 4))      // avg of nothing
}

func TestCompute_Arity(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"abs(A1, C1)":       string(sheet.ErrValue),
		"len(A1, C1)":       string(sheet.ErrValue),
		"round()":           string(sheet.ErrValue),
		"round(A1, C1, D1)": string(sheet.ErrValue),
		"if(A1)":            string(sheet.ErrValue),
		"bogus(A1)":         string(sheet.ErrName),
		"round(A1, Z99)":    string(sheet.ErrRef), // error place argument
	}
	for expr, want := range cases {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, formula1(t, expr))
		})
	}
}

func TestCompute_A1Forms(t *testing.T) {
	t.Parallel()

	// A1=2, C1=3, D1=4 (B1 holds the formula).
	assert.Equal(t, "2", formula1(t, "$A$1")) // absolute-marked
	assert.Equal(t, "2", formula1(t, "A$1"))  // row-absolute form
	assert.Equal(t, "2", formula1(t, "A1"))   // plain
}

func TestCompute_RowZeroIsRef(t *testing.T) {
	t.Parallel()

	// Row 0 is below the grid, so `A0` is #REF! (the grammar admits the syntax;
	// resolution rejects the row).
	assert.Equal(t, string(sheet.ErrRef), formula1(t, "A0"))
	assert.Equal(t, string(sheet.ErrRef), formula1(t, "sum(A0:A0)"))
}

func TestCompute_RangeInScalarContext(t *testing.T) {
	t.Parallel()

	// A range where a single value is required is #VALUE!.
	assert.Equal(t, string(sheet.ErrValue), formula1(t, "A2:C2 + 1"))
}

func TestCompute_BuiltinsPropagateErrors(t *testing.T) {
	t.Parallel()

	// Z98:Z99 (and Z99) are out of grid, so each builtin propagates #REF!.
	for _, expr := range []string{
		"min(Z98:Z99)", "max(Z98:Z99)", "count(Z98:Z99)", "avg(Z98:Z99)",
		"abs(Z99)", "round(Z99)", "concat(A1, Z99)", "len(Z99)",
	} {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, string(sheet.ErrRef), formula1(t, expr))
		})
	}
}

func TestCompute_StringLeftArithmetic(t *testing.T) {
	t.Parallel()

	// A text cell on the left of arithmetic is #VALUE!.
	g := compute(t, "apple\t=A1 + 1\n")
	assert.Equal(t, string(sheet.ErrValue), cellAt(t, g, 0, 1))
}

func TestCompute_EmptyComparedToText(t *testing.T) {
	t.Parallel()

	// Comparing an empty cell to a text cell exercises the empty-text path.
	g := compute(t, "\tx\t=if(A1 = B1, 1, 0)\n") // A1 empty, B1="x"
	assert.Equal(t, "0", cellAt(t, g, 0, 2))
}

func TestCompute_ReversedRange(t *testing.T) {
	t.Parallel()

	// A range written high-to-low spans the same hull (ordered corners).
	g := compute(t, "1\t2\t3\t=sum(C1:A1)\n") // C1:A1 == A1:C1
	assert.Equal(t, "6", cellAt(t, g, 0, 3))
}

func TestParse_ReadError(t *testing.T) {
	t.Parallel()

	// A single line exceeding the scanner's 1 MiB bound surfaces a read error.
	huge := make([]byte, 2<<20)
	for i := range huge {
		huge[i] = 'x'
	}
	_, err := sheet.Parse(huge)
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrReadInput)
}
