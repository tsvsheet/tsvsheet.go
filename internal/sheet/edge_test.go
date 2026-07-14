package sheet_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/sheet"
	"github.com/uplang/tsvsheet.go/internal/tsvt"
)

// evalData computes `Z=<formula>` against custom data and returns row 0's Z.
func evalData(t *testing.T, formula, data string) string {
	t.Helper()
	out := computeGrid(t, "=body\nZ="+formula, data)
	return out[0][25]
}

func TestValue_EmptyCell(t *testing.T) {
	t.Parallel()

	// Column B is empty in the data.
	const data = "1\t\t3\n"
	assert.Equal(t, "", evalData(t, "B", data))       // empty renders empty
	assert.Equal(t, "1", evalData(t, "1 + B", data))  // empty is 0 in arithmetic
	assert.Equal(t, "0", evalData(t, "sum(B)", data)) // sum of only-empty is 0
}

func TestValue_EmptyExcludedFromCount(t *testing.T) {
	t.Parallel()

	const data = "1\t\t3\n"
	// count(A:C) counts A and C (B empty excluded) = 2.
	assert.Equal(t, "2", evalData(t, "count(A:C)", data))
}

func TestValue_ErrorCellPropagates(t *testing.T) {
	t.Parallel()

	// A data cell holding an error value round-trips and propagates.
	const data = "#REF!\t2\n"
	assert.Equal(t, string(sheet.ErrRef), evalData(t, "A", data))
	assert.Equal(t, string(sheet.ErrRef), evalData(t, "A + B", data))
	assert.Equal(t, string(sheet.ErrRef), evalData(t, "sum(A:B)", data))
	assert.Equal(t, string(sheet.ErrRef), evalData(t, "count(A:B)", data))
	assert.Equal(t, string(sheet.ErrRef), evalData(t, "concat(A, B)", data))
	assert.Equal(t, string(sheet.ErrRef), evalData(t, "len(A)", data))
	assert.Equal(t, string(sheet.ErrRef), evalData(t, "abs(A)", data))
	assert.Equal(t, string(sheet.ErrRef), evalData(t, "round(A)", data))
}

func TestValue_StringComparison(t *testing.T) {
	t.Parallel()

	// Unbound named columns are string literals (rule 16); compare them.
	assert.Equal(t, "1", eval1(t, `if("apple" < "banana", 1, 0)`))
	assert.Equal(t, "0", eval1(t, `if("banana" < "apple", 1, 0)`))
	assert.Equal(t, "1", eval1(t, `if("apple" = "apple", 1, 0)`))
	assert.Equal(t, "1", eval1(t, `if("apple" <> "banana", 1, 0)`))
	assert.Equal(t, "1", eval1(t, `if("banana" > "apple", 1, 0)`))
	assert.Equal(t, "1", eval1(t, `if("apple" <= "apple", 1, 0)`))
	assert.Equal(t, "1", eval1(t, `if("apple" >= "apple", 1, 0)`))
}

func TestValue_MixedComparisonIsValueError(t *testing.T) {
	t.Parallel()

	// Comparing a number to a string is #VALUE! (rule 12).
	assert.Equal(t, string(sheet.ErrValue), eval1(t, `C < "apple"`))
}

func TestValue_StringTruthy(t *testing.T) {
	t.Parallel()

	// A non-empty string is truthy; an empty string is falsy (rule 9).
	assert.Equal(t, "1", eval1(t, `if("x", 1, 0)`))
	assert.Equal(t, "0", evalData(t, `if(B, 1, 0)`, "1\t\t3\n")) // B empty → falsy
}

func TestValue_NonNumericArithmeticIsValueError(t *testing.T) {
	t.Parallel()

	// Arithmetic on a string operand is #VALUE!.
	assert.Equal(t, string(sheet.ErrValue), eval1(t, `1 + "apple"`))
}

func TestBuiltins_EmptyAggregates(t *testing.T) {
	t.Parallel()

	const data = "1\t\t3\n"
	// min/max over an only-empty range have no numeric values → #VALUE!.
	assert.Equal(t, string(sheet.ErrValue), evalData(t, "min(B)", data))
	assert.Equal(t, string(sheet.ErrValue), evalData(t, "max(B)", data))
	// avg over an only-empty range → #DIV/0!.
	assert.Equal(t, string(sheet.ErrDiv), evalData(t, "avg(B)", data))
}

func TestBuiltins_Arity(t *testing.T) {
	t.Parallel()

	// Wrong-arity builtins are #VALUE!.
	assert.Equal(t, string(sheet.ErrValue), eval1(t, "abs(C, D)"))
	assert.Equal(t, string(sheet.ErrValue), eval1(t, "len(C, D)"))
	assert.Equal(t, string(sheet.ErrValue), eval1(t, "round()"))
	assert.Equal(t, string(sheet.ErrValue), eval1(t, "round(C, D, A)"))
	assert.Equal(t, string(sheet.ErrValue), eval1(t, "if(C)"))
}

func TestBuiltins_RoundErrorPlaces(t *testing.T) {
	t.Parallel()

	// A #REF! place argument propagates.
	assert.Equal(t, string(sheet.ErrRef), evalAt(t, "round(C, C2)", 1)) // C2 → #REF!
}

func TestCheck_UnknownFunctionAdvisory(t *testing.T) {
	t.Parallel()

	tmpl, err := tsvt.Parse(tsvt.Source("=body\nZ=bogus(C)\nY=sum(A)"))
	require.NoError(t, err)
	diags := sheet.Check(tmpl)
	require.Len(t, diags, 1)
	assert.False(t, diags[0].Fatal)
	assert.Contains(t, diags[0].Message, "bogus")
}

func TestCheck_UnknownFunctionInPayload(t *testing.T) {
	t.Parallel()

	tmpl, err := tsvt.Parse(tsvt.Source("=final\nZ$=bogus(A)"))
	require.NoError(t, err)
	diags := sheet.Check(tmpl)
	require.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "bogus")
}

func TestCheck_PerCellModifierFatal(t *testing.T) {
	t.Parallel()

	// A per-cell structural modifier is rejected (rule 18).
	tmpl, err := tsvt.Parse(tsvt.Source("=body\nC!"))
	require.NoError(t, err)
	diags := sheet.Check(tmpl)
	require.Len(t, diags, 1)
	assert.True(t, diags[0].Fatal)
}

func TestCheck_StructuralWithRowFatal(t *testing.T) {
	t.Parallel()

	// A structural command with a row (per-cell scope) is rejected.
	tmpl, err := tsvt.Parse(tsvt.Source("=final\n=C1<"))
	require.NoError(t, err)
	diags := sheet.Check(tmpl)
	require.Len(t, diags, 1)
	assert.True(t, diags[0].Fatal)
}

func TestCheck_CleanTemplate(t *testing.T) {
	t.Parallel()

	tmpl, err := tsvt.Parse(tsvt.Source("=body\nZ=C + D"))
	require.NoError(t, err)
	assert.Empty(t, sheet.Check(tmpl))
}

func TestCompute_NoData(t *testing.T) {
	t.Parallel()

	// A final aggregate over an empty grid: no rows, references are #REF!.
	tmpl, err := tsvt.Parse(tsvt.Source("=body\nZ=C"))
	require.NoError(t, err)
	out, err := sheet.Compute(tmpl, sheet.Grid{})
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestCompute_NamedFromRangeNonNamed(t *testing.T) {
	t.Parallel()

	// A header cell that is a reference but not a named column (a letter ref)
	// binds no name; the formula referencing it by letter still works.
	out := computeGrid(t, "=header(1)\nA\tB\tC$1\tD\n=body\nZ=A", fixedData)
	assert.Equal(t, "1", out[0][25])
}

func TestCompute_MatrixFromMixed(t *testing.T) {
	t.Parallel()

	// A reversed matrix (endpoints out of order) still spans the hull.
	assert.Equal(t, "15", eval1(t, "sum(B$3:A$1)")) // same 6 cells as A$1:B$3
}

func TestReadTSV_LongLine(t *testing.T) {
	t.Parallel()

	// A line at the buffer boundary still reads.
	long := strings.Repeat("x", 1000)
	g, err := sheet.ReadTSV(strings.NewReader(long + "\n"))
	require.NoError(t, err)
	assert.Equal(t, sheet.Grid{{long}}, g)
}
