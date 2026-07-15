package sheet_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/constants"
	"github.com/uplang/tsvsheet.go/internal/sheet"
)

// compute parses a .tsvt grid and returns the computed grid.
func compute(t *testing.T, src string) sheet.Grid {
	t.Helper()
	s, err := sheet.Parse([]byte(src))
	require.NoError(t, err)
	return s.Compute()
}

// cellAt reads the computed value at a 0-based (row, col).
func cellAt(t *testing.T, g sheet.Grid, row, col int) string {
	t.Helper()
	require.Less(t, row, len(g))
	require.Less(t, col, len(g[row]))
	return g[row][col]
}

// formula1 puts a single formula in cell B1 over a fixed first row of data and
// returns its computed value. The first row is: A1=2 B1=<formula> C1=3 D1=4;
// row 2 provides A2=5 B2=6 C2=7 D2=8 for cross-row references.
func formula1(t *testing.T, expr string) string {
	t.Helper()
	src := "2\t=" + expr + "\t3\t4\n5\t6\t7\t8\n"
	return cellAt(t, compute(t, src), 0, 1)
}

func TestCompute_UserExample(t *testing.T) {
	t.Parallel()

	src := "name\tscore\tbonus\ttotal\n" +
		"Alice\t85\t5\t=B2+C2\n" +
		"Bob\t72\t10\t=B3+C3\n" +
		"Carol\t95\t0\t=B4+C4\n" +
		"\t\ttop:\t=max(D2:D4)\n"
	g := compute(t, src)
	assert.Equal(t, "90", cellAt(t, g, 1, 3))    // D2 = B2+C2
	assert.Equal(t, "82", cellAt(t, g, 2, 3))    // D3
	assert.Equal(t, "95", cellAt(t, g, 3, 3))    // D4
	assert.Equal(t, "95", cellAt(t, g, 4, 3))    // max(D2:D4)
	assert.Equal(t, "Alice", cellAt(t, g, 1, 0)) // literal verbatim
}

func TestCompute_LiteralsVerbatim(t *testing.T) {
	t.Parallel()

	// Literals pass through exactly — including a decimal that a number would
	// normalize and text.
	g := compute(t, "4.50\thello\t\t=A1\n")
	assert.Equal(t, "4.50", cellAt(t, g, 0, 0)) // literal kept verbatim
	assert.Equal(t, "hello", cellAt(t, g, 0, 1))
	assert.Equal(t, "", cellAt(t, g, 0, 2))    // empty literal
	assert.Equal(t, "4.5", cellAt(t, g, 0, 3)) // formula reads it as a number
}

func TestCompute_Arithmetic(t *testing.T) {
	t.Parallel()

	// B1=<formula>; A1=2, C1=3, D1=4.
	cases := map[string]string{
		"A1 + C1":        "5",
		"D1 - C1":        "1",
		"C1 * D1":        "12",
		"D1 / A1":        "2",
		"-A1":            "-2",
		"+A1":            "2",
		"C1 + D1 * A1":   "11", // precedence: 3 + (4*2)
		"(C1 + D1) * A1": "14",
		"5":              "5",
		"10 + A1":        "12", // literal-first addition
	}
	for expr, want := range cases {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, formula1(t, expr))
		})
	}
}

func TestCompute_Comparison(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"A1 = A1":  "TRUE",
		"A1 = C1":  "FALSE",
		"A1 <> C1": "TRUE",
		"A1 < C1":  "TRUE",
		"A1 <= A1": "TRUE",
		"D1 > C1":  "TRUE",
		"D1 >= D1": "TRUE",
		"D1 < C1":  "FALSE",
		// A comparison of two booleans compares their 1/0 values.
		"(A1 < C1) = (D1 > C1)":  "TRUE",
		"(A1 < C1) <> (D1 < C1)": "TRUE",
	}
	for expr, want := range cases {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, formula1(t, expr))
		})
	}
}

func TestCompute_BooleanCoercion(t *testing.T) {
	t.Parallel()

	// A1=2, C1=3. A boolean coerces to 1/0 in arithmetic.
	assert.Equal(t, "1", formula1(t, "(A1 < C1) + 0")) // TRUE + 0
	assert.Equal(t, "0", formula1(t, "(A1 > C1) * 5")) // FALSE * 5
}

func TestCompute_AverageAlias(t *testing.T) {
	t.Parallel()

	// A2=5 B2=6 C2=7. `average` is an alias of `avg`.
	assert.Equal(t, "6", formula1(t, "average(A2:C2)"))
}

func TestCompute_NonNumericIsValueError(t *testing.T) {
	t.Parallel()

	// A1 holds text; every numeric operation over it is #VALUE!.
	cases := map[string]string{
		"=sum(A1)":      "#VALUE!",
		"=min(A1)":      "#VALUE!",
		"=max(A1)":      "#VALUE!",
		"=avg(A1)":      "#VALUE!",
		"=abs(A1)":      "#VALUE!",
		"=round(A1)":    "#VALUE!",
		"=round(2, A1)": "#VALUE!", // non-numeric place count
		"=mod(A1, 2)":   "#VALUE!", // non-numeric dividend
		"=mod(2, A1)":   "#VALUE!", // non-numeric divisor
		"=A1%":          "#VALUE!", // percent of non-numeric
	}
	for expr, want := range cases {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, cellAt(t, compute(t, "hi\t"+expr+"\n"), 0, 1))
		})
	}
}

func TestCompute_DivZero(t *testing.T) {
	t.Parallel()

	assert.Equal(t, string(sheet.ErrDiv), formula1(t, "A1 / 0"))
	assert.Equal(t, string(sheet.ErrDiv), formula1(t, "mod(A1, 0)"))
}

func TestCompute_ExcelOperators(t *testing.T) {
	t.Parallel()

	// A1=2, C1=3, D1=4 (B1 holds the formula).
	cases := map[string]string{
		"2 ^ 8":             "256",  // power
		"D1 ^ A1":           "16",   // 4^2
		"50%":               "0.5",  // postfix percent
		"A1 * 50%":          "1",    // 2 * 0.5
		"-2 ^ 2":            "-4",   // ^ binds tighter than unary minus
		`"a" & "b"`:         "ab",   // text concatenation
		"A1 & C1":           "23",   // number operands concatenate as text
		"mod(D1, C1)":       "1",    // MOD replaces the retired binary %
		"mod(-7, 3)":        "-1",   // truncated toward zero
		"TRUE":              "TRUE", // boolean literal
		"FALSE":             "FALSE",
		"if(TRUE, A1, C1)":  "2",
		"if(FALSE, A1, C1)": "3",
	}
	for expr, want := range cases {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, formula1(t, expr))
		})
	}
}

func TestCompute_StringAndErrorLiterals(t *testing.T) {
	t.Parallel()

	// A quoted string is a string value (the former #REF! gap is closed); an
	// error-value literal is that error value and propagates.
	assert.Equal(t, "Pass", formula1(t, `if(A1 >= 1, "Pass", "Fail")`))
	assert.Equal(t, "hello world", formula1(t, `"hello world"`))
	assert.Equal(t, string(sheet.ErrNA), formula1(t, "#N/A"))
	assert.Equal(t, string(sheet.ErrNA), formula1(t, "#N/A + 1")) // propagates
}

func TestCompute_Functions(t *testing.T) {
	t.Parallel()

	// A1=2 C1=3 D1=4; A2=5 B2=6 C2=7 D2=8.
	cases := map[string]string{
		"sum(A2:C2)":          "18", // 5+6+7 (avoids B1, the formula cell itself)
		"min(A2:D2)":          "5",
		"max(A2:D2)":          "8",
		"count(A2:D2)":        "4",
		"avg(A2:D2)":          "6.5",
		"abs(-D1)":            "4",
		"round(D1 / A1, 1)":   "2",
		"round(A1)":           "2", // single arg → 0 decimal places
		"round(2.7)":          "3",
		"if(A1 < C1, A1, C1)": "2",
		"if(A1 > C1, A1, C1)": "3",
		"if(0, A1, C1)":       "3",
		"len(D1)":             "1",
		"sum(1, 2, 3)":        "6",
		"SUM(A2:D2)":          "26", // case-insensitive
		"concat(A1, C1)":      "23",
	}
	for expr, want := range cases {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, formula1(t, expr))
		})
	}
}

func TestCompute_ChainedReferences(t *testing.T) {
	t.Parallel()

	// A1=1, B1=A1+10, C1=B1*2 → resolved in dependency order regardless of
	// evaluation order.
	g := compute(t, "1\t=A1 + 10\t=B1 * 2\n")
	assert.Equal(t, "11", cellAt(t, g, 0, 1))
	assert.Equal(t, "22", cellAt(t, g, 0, 2))
}

func TestCompute_CircularReference(t *testing.T) {
	t.Parallel()

	// A1 -> B1 -> A1 is circular.
	g := compute(t, "=B1 + 1\t=A1 + 1\n")
	assert.Equal(t, string(sheet.ErrCirc), cellAt(t, g, 0, 0))
	assert.Equal(t, string(sheet.ErrCirc), cellAt(t, g, 0, 1))
}

func TestCompute_SelfReference(t *testing.T) {
	t.Parallel()

	assert.Equal(t, string(sheet.ErrCirc), cellAt(t, compute(t, "=A1 + 1\n"), 0, 0))
}

func TestCompute_OutOfGrid(t *testing.T) {
	t.Parallel()

	assert.Equal(t, string(sheet.ErrRef), formula1(t, "Z99"))
	assert.Equal(t, string(sheet.ErrRef), formula1(t, "sum(A5:A9)")) // rows past the grid
}

func TestCompute_ErrorPropagation(t *testing.T) {
	t.Parallel()

	assert.Equal(t, string(sheet.ErrRef), formula1(t, "Z99 + A1"))        // left
	assert.Equal(t, string(sheet.ErrRef), formula1(t, "A1 + Z99"))        // right
	assert.Equal(t, string(sheet.ErrRef), formula1(t, "-Z99"))            // unary
	assert.Equal(t, string(sheet.ErrRef), formula1(t, "sum(Z99)"))        // aggregate
	assert.Equal(t, string(sheet.ErrRef), formula1(t, "if(Z99, A1, C1)")) // condition
}

func TestCompute_ErrorLiteralPropagates(t *testing.T) {
	t.Parallel()

	// A cell literally holding an error value round-trips and propagates.
	g := compute(t, "#REF!\t=A1 + 1\n")
	assert.Equal(t, string(sheet.ErrRef), cellAt(t, g, 0, 1))
}

func TestParse_SyntaxErrorNamesCell(t *testing.T) {
	t.Parallel()

	_, err := sheet.Parse([]byte("1\t2\n3\t=sum(\n")) // B2 malformed
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrSyntax)
	assert.Contains(t, err.Error(), "B2")
}
