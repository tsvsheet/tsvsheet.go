package sheet_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/constants"
	"github.com/uplang/tsvsheet.go/internal/sheet"
)

// parse is a test helper that parses a sheet, failing on error.
func parse(t *testing.T, src string) sheet.Sheet {
	t.Helper()
	s, err := sheet.Parse([]byte(src))
	require.NoError(t, err)
	return s
}

func TestCheck_Clean(t *testing.T) {
	t.Parallel()

	assert.Empty(t, sheet.Check(parse(t, "1\t2\t=A1 + B1\n")))
}

func TestCheck_UnknownFunction(t *testing.T) {
	t.Parallel()

	diags := sheet.Check(parse(t, "1\t=bogus(A1)\n"))
	require.Len(t, diags, 1)
	assert.Equal(t, "B1", diags[0].Cell)
	assert.Contains(t, diags[0].Message, "bogus")
	assert.False(t, diags[0].IsFatal)
}

func TestCheck_NumberFormulaHasNoRefs(t *testing.T) {
	t.Parallel()

	// A formula with no calls yields no diagnostics (the walker no-ops).
	assert.Empty(t, sheet.Check(parse(t, "=1 + 2\n")))
}

func TestCheck_KnownFunctionsClean(t *testing.T) {
	t.Parallel()

	// `if` (dispatched specially) and a registered function (`sum`) are both
	// known, so a formula built from them raises no diagnostics.
	assert.Empty(t, sheet.Check(parse(t, "1\t2\t=if(A1 > B1, sum(A1:B1), 0)\n")))
}

func TestCheck_WalksIntoUnaryPercentBinaryAndCall(t *testing.T) {
	t.Parallel()

	// Each wrapper form must be walked to reach the unknown call inside it.
	for _, src := range []string{"=-bogus(A1)", "=bogus(A1)%", "=bogus(A1) + 1", "=abs(bogus(A1))"} {
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			diags := sheet.Check(parse(t, "1\t"+src+"\n"))
			require.Len(t, diags, 1)
			assert.Contains(t, diags[0].Message, "bogus")
		})
	}
}

func TestExplain_Formula(t *testing.T) {
	t.Parallel()

	// C1 = A1 + B1 over 2 and 3.
	trace, err := sheet.Explain(parse(t, "2\t3\t=A1 + B1\n"), sheet.Address{Row: 0, Col: 2})
	require.NoError(t, err)
	assert.Equal(t, "C1", trace.Cell)
	assert.Equal(t, "5", trace.Value)
	assert.Equal(t, "A1 + B1", trace.Formula)
	assert.Equal(t, []sheet.TraceInput{{Ref: "A1", Value: "2"}, {Ref: "B1", Value: "3"}}, trace.Inputs)
}

func TestExplain_RangeInput(t *testing.T) {
	t.Parallel()

	// A range operand renders as a two-cell A1 range in the trace.
	trace, err := sheet.Explain(parse(t, "1\t2\t=sum(A1:B1)\n"), sheet.Address{Row: 0, Col: 2})
	require.NoError(t, err)
	require.Len(t, trace.Inputs, 1)
	assert.Equal(t, "A1:B1", trace.Inputs[0].Ref)
}

func TestExplain_Literal(t *testing.T) {
	t.Parallel()

	trace, err := sheet.Explain(parse(t, "hello\t=A1\n"), sheet.Address{Row: 0, Col: 0})
	require.NoError(t, err)
	assert.Equal(t, "hello", trace.Value)
	assert.Empty(t, trace.Formula)
	assert.Empty(t, trace.Inputs)
}

func TestExplain_OutOfGrid(t *testing.T) {
	t.Parallel()

	_, err := sheet.Explain(parse(t, "1\t2\n"), sheet.Address{Row: 9, Col: 9})
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrNotFound)
}

func TestExplain_RendersEveryExpressionForm(t *testing.T) {
	t.Parallel()

	// Each formula exercises one RenderExpr branch; the rendered form round-trips.
	cases := map[string]string{
		"=42":      "42",
		`="hi"`:    `"hi"`,
		"=TRUE":    "TRUE",
		"=FALSE":   "FALSE",
		"=#N/A":    "#N/A",
		"=-A1":     "-A1",
		"=A1%":     "A1%",
		"=A1 + 1":  "A1 + 1",
		"=abs(A1)": "abs(A1)",
	}
	for src, want := range cases {
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			trace, err := sheet.Explain(parse(t, "5\t"+src+"\n"), sheet.Address{Row: 0, Col: 1})
			require.NoError(t, err)
			assert.Equal(t, want, trace.Formula)
		})
	}
}
