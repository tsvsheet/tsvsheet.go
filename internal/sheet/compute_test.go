package sheet_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/sheet"
	"github.com/uplang/tsvsheet.go/internal/tsvt"
)

// computeGrid parses a template and data grid and returns the computed grid.
func computeGrid(t *testing.T, template, data string) sheet.Grid {
	t.Helper()
	tmpl, err := tsvt.Parse(tsvt.Source(template))
	require.NoError(t, err)
	g, err := sheet.ReadTSV(strings.NewReader(data))
	require.NoError(t, err)
	out, err := sheet.Compute(tmpl, g)
	require.NoError(t, err)
	return out
}

// fixedData is a 3×4 grid (columns A–D):
//
//	1 2 3 4
//	2 3 4 5
//	3 4 5 6
const fixedData = "1\t2\t3\t4\n2\t3\t4\t5\n3\t4\t5\t6\n"

// evalAt computes `Z=<formula>` (column Z, index 25) against fixedData and
// returns the value at the given data row.
func evalAt(t *testing.T, formula string, row int) string {
	t.Helper()
	out := computeGrid(t, "=body\nZ="+formula, fixedData)
	return out[row][25]
}

// eval1 computes a formula at the middle row (row 1: A=2,B=3,C=4,D=5).
func eval1(t *testing.T, formula string) string {
	t.Helper()
	return evalAt(t, formula, 1)
}

func TestEval_Arithmetic(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"C + D":       "9",
		"D - C":       "1",
		"C * D":       "20",
		"D / C":       "1.25",
		"D % C":       "1",
		"-C":          "-4",
		"+C":          "4",
		"C + D * A":   "14", // 4 + 5*2, precedence
		"(C + D) * A": "18",
		"5":           "5",
		"3.5 + 1":     "4.5",
	}
	for formula, want := range cases {
		t.Run(formula, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, eval1(t, formula))
		})
	}
}

func TestEval_Comparison(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"C = C":  "1",
		"C = D":  "0",
		"C <> D": "1",
		"C < D":  "1",
		"C <= C": "1",
		"D > C":  "1",
		"D >= D": "1",
		"D < C":  "0",
	}
	for formula, want := range cases {
		t.Run(formula, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, eval1(t, formula))
		})
	}
}

func TestEval_DivZero(t *testing.T) {
	t.Parallel()

	assert.Equal(t, string(sheet.ErrDiv), eval1(t, "C / 0"))
	assert.Equal(t, string(sheet.ErrDiv), eval1(t, "C % 0"))
}

func TestEval_Functions(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"sum(A:D)":        "14", // 2+3+4+5
		"min(A:D)":        "2",
		"max(A:D)":        "5",
		"count(A:D)":      "4",
		"avg(A:D)":        "3.5",
		"abs(-C)":         "4",
		"abs(0 - D)":      "5",
		"round(D / C, 1)": "1.3", // 1.25 → 1.3
		"round(D / C)":    "1",
		"if(C < D, C, D)": "4",
		"if(C > D, C, D)": "5",
		"if(0, C, D)":     "5",
		"len(D)":          "1",
		"sum(1, 2, 3)":    "6",
		"SUM(A:D)":        "14", // case-insensitive
	}
	for formula, want := range cases {
		t.Run(formula, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, eval1(t, formula))
		})
	}
}

func TestEval_Concat(t *testing.T) {
	t.Parallel()

	// Concat of numeric cells renders their string forms.
	assert.Equal(t, "45", eval1(t, "concat(C, D)"))
}

func TestEval_UnknownFunctionValue(t *testing.T) {
	t.Parallel()

	// An unknown function computes to #NAME? (and Check flags it; see Check
	// tests). Compute does not reject it because the diagnostic is advisory.
	assert.Equal(t, string(sheet.ErrName), eval1(t, "bogus(C)"))
}

func TestEval_OutOfGridRef(t *testing.T) {
	t.Parallel()

	// C1 at row 0 is one row before the grid → #REF!.
	assert.Equal(t, string(sheet.ErrRef), evalAt(t, "C1", 0))
	// A column far past the grid → #REF!.
	assert.Equal(t, string(sheet.ErrRef), eval1(t, "AZ"))
}

func TestEval_ErrorPropagation(t *testing.T) {
	t.Parallel()

	// C2 at row 1 resolves to row -1, out of grid → #REF!.
	assert.Equal(t, string(sheet.ErrRef), eval1(t, "C2 + D"))       // left error
	assert.Equal(t, string(sheet.ErrRef), eval1(t, "D + C2"))       // right error
	assert.Equal(t, string(sheet.ErrRef), eval1(t, "-C2"))          // unary error
	assert.Equal(t, string(sheet.ErrRef), eval1(t, "sum(C2)"))      // aggregate error
	assert.Equal(t, string(sheet.ErrRef), eval1(t, "if(C2, C, D)")) // condition error
}
