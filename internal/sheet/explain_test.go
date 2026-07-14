package sheet_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/constants"
	"github.com/uplang/tsvsheet.go/internal/sheet"
	"github.com/uplang/tsvsheet.go/internal/tsvt"
)

// explain parses, reads, and explains the cell at addr.
func explain(t *testing.T, template, data, addr string) sheet.Trace {
	t.Helper()
	tmpl, err := tsvt.Parse(tsvt.Source(template))
	require.NoError(t, err)
	g, err := sheet.ReadTSV(strings.NewReader(data))
	require.NoError(t, err)
	at, err := sheet.ParseAddress(addr)
	require.NoError(t, err)
	trace, err := sheet.Explain(tmpl, g, at)
	require.NoError(t, err)
	return trace
}

func TestExplain_BodyFormula(t *testing.T) {
	t.Parallel()

	// Z at row 2 (address Z2) is produced by `Z=C + D`; at row 1 C=4, D=5.
	trace := explain(t, "=body\nZ=C + D", fixedData, "Z2")
	assert.Equal(t, "9", trace.Value)
	assert.Equal(t, "C + D", trace.Formula)
	assert.Equal(t, []sheet.TraceInput{{Ref: "C", Value: "4"}, {Ref: "D", Value: "5"}}, trace.Inputs)
}

func TestExplain_FinalPlacement(t *testing.T) {
	t.Parallel()

	// B at the last row is produced by the final aggregate.
	trace := explain(t, "=final\nB$=sum(B$1:B$3)", fixedData, "B3")
	assert.Equal(t, "9", trace.Value)
	assert.Equal(t, "sum(B$1:B$3)", trace.Formula)
	require.Len(t, trace.Inputs, 1)
	assert.Equal(t, "B$1:B$3", trace.Inputs[0].Ref)
}

func TestExplain_PlainDataCell(t *testing.T) {
	t.Parallel()

	// A1 is raw data with no producing formula.
	trace := explain(t, "=body\nZ=C", fixedData, "A1")
	assert.Equal(t, "1", trace.Value)
	assert.Empty(t, trace.Formula)
	assert.Empty(t, trace.Inputs)
}

func TestExplain_FinalWithStructural(t *testing.T) {
	t.Parallel()

	// A structural command in the final section is skipped by the formula
	// lookup (it is not a cell row).
	trace := explain(t, "=final\n=A<\nC$=sum(A$1:A$3)", fixedData, "C3")
	assert.Equal(t, "sum(A$1:A$3)", trace.Formula)
}

func TestExplain_OutOfGrid(t *testing.T) {
	t.Parallel()

	tmpl, err := tsvt.Parse(tsvt.Source("=body\nZ=C"))
	require.NoError(t, err)
	g, err := sheet.ReadTSV(strings.NewReader(fixedData))
	require.NoError(t, err)

	_, err = sheet.Explain(tmpl, g, sheet.Address{Row: 99, Col: 0})
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrNotFound)
}

func TestExplain_FatalTemplate(t *testing.T) {
	t.Parallel()

	tmpl, err := tsvt.Parse(tsvt.Source("=final\n=A:C<"))
	require.NoError(t, err)
	g, err := sheet.ReadTSV(strings.NewReader(fixedData))
	require.NoError(t, err)

	_, err = sheet.Explain(tmpl, g, sheet.Address{Row: 0, Col: 0})
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrUnsupported)
}

// TestExplain_RendersAllReferenceForms drives every reference/row/column render
// branch through a single formula's trace inputs.
func TestExplain_RendersAllReferenceForms(t *testing.T) {
	t.Parallel()

	const formula = `Z=sum(A, $B, $, "X", [3], [,$], C1, C+1, E*, C$, C$+1, C$-1, C$3, [0,-1], (A:C)0, A$1:B$2, *)`
	trace := explain(t, "=body\n"+formula, fixedData, "Z1")

	rendered := make([]string, len(trace.Inputs))
	for i, in := range trace.Inputs {
		rendered[i] = in.Ref
	}
	assert.Equal(t, []string{
		"A", "$B", "$", `"X"`, "[3]", "[,$]", "C1", "C+1", "E*", "C$", "C$+1", "C$-1", "C$3", "[0,-1]", "(A:C)0", "A$1:B$2", "*",
	}, rendered)
}
