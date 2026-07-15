package sheet_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uplang/tsvsheet.go/internal/sheet"
)

func TestStat_Aggregates(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"median(1, 2, 3, 4)":      "2.5", // even
		"median(1, 5, 2)":         "2",   // odd
		"mode(1, 2, 2, 3)":        "2",
		"stdev(2, 4, 6)":          "2",
		"stdevp(2, 4, 6)":         "1.632993161855452",
		"var(2, 4, 6)":            "4",
		"varp(2, 4, 6)":           "2.6666666666666665",
		"geomean(1, 4, 16)":       "4",
		"large(5, 1, 3, 2, 4, 2)": "4", // 2nd largest of {5,1,3,2,4}
		"small(5, 1, 3, 2, 1)":    "1", // smallest
		"counta(A2:C2)":           "3", // filled cells (data row 2)
	}
	for expr, want := range cases {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, formula1(t, expr))
		})
	}
}

func TestStat_Edges(t *testing.T) {
	t.Parallel()

	assert.Equal(t, string(sheet.ErrNA), formula1(t, "mode(1, 2, 3)"))   // no repeat
	assert.Equal(t, string(sheet.ErrNum), formula1(t, "geomean(1, -4)")) // non-positive
	assert.Equal(t, string(sheet.ErrNum), formula1(t, "large(1, 2, 9)")) // k out of range
	assert.Equal(t, string(sheet.ErrNum), formula1(t, "small(1, 2, 9)"))
	assert.Equal(t, "0", formula1(t, "countblank(A2:C2)"))                   // row 2 is filled
	assert.Equal(t, "1", cellAt(t, compute(t, "\t=countblank(A1)\n"), 0, 1)) // A1 empty
	// Empty sets: median/geomean over an empty cell → #NUM!; stdev of one value
	// → #DIV/0!.
	assert.Equal(t, string(sheet.ErrNum), cellAt(t, compute(t, "\t=median(A1)\n"), 0, 1))
	assert.Equal(t, string(sheet.ErrNum), cellAt(t, compute(t, "\t=geomean(A1)\n"), 0, 1))
	assert.Equal(t, string(sheet.ErrDiv), formula1(t, "stdev(5)")) // sample needs >= 2
}

func TestStat_NonNumeric(t *testing.T) {
	t.Parallel()

	// A1 is text; each aggregate reports #VALUE!.
	for _, expr := range []string{
		"=median(A1)", "=mode(A1)", "=stdev(A1, A1)", "=geomean(A1)", "=large(A1, 1)", "=large(1, 2, A1)",
	} {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, "#VALUE!", cellAt(t, compute(t, "hi\t"+expr+"\n"), 0, 1))
		})
	}
}

func TestStat_Criteria(t *testing.T) {
	t.Parallel()

	// A1:C1 = 10,20,30; D1:F1 = 1,2,3 (a parallel sum range).
	g := compute(t, "10\t20\t30\t1\t2\t3\n"+
		"=countif(A1:C1, \">15\")\t=sumif(A1:C1, \">15\")\t=averageif(A1:C1, \">=20\")\t"+
		"=countif(A1:C1, 20)\t=sumif(A1:C1, \">15\", D1:F1)\t=countif(A1:C1, \">abc\")\n")
	assert.Equal(t, "2", cellAt(t, g, 1, 0))  // 20, 30 > 15
	assert.Equal(t, "50", cellAt(t, g, 1, 1)) // 20 + 30
	assert.Equal(t, "25", cellAt(t, g, 1, 2)) // (20+30)/2
	assert.Equal(t, "1", cellAt(t, g, 1, 3))  // exactly 20
	assert.Equal(t, "5", cellAt(t, g, 1, 4))  // D1,E1 for matches at cols 2,3 → 2+3
	assert.Equal(t, "0", cellAt(t, g, 1, 5))  // ">abc" parses no number → no match
}

func TestStat_CriteriaEdges(t *testing.T) {
	t.Parallel()

	// averageif with no match is #DIV/0!; a short sum range skips overflow
	// positions; a criterion comparison on a text cell never matches.
	g := compute(t, "10\t20\t30\t1\t2\n"+ // A1:C1 data, D1:E1 a short sum range
		"=averageif(A1:C1, \">100\")\t=sumif(A1:C1, \">5\", D1:E1)\t=countif(A1:B1, 20, 1)\t=sumif(A1:C1, \">5\")\n")
	assert.Equal(t, string(sheet.ErrDiv), cellAt(t, g, 1, 0))   // no match
	assert.Equal(t, "3", cellAt(t, g, 1, 1))                    // D1+E1 (col3 has no sum cell)
	assert.Equal(t, string(sheet.ErrValue), cellAt(t, g, 1, 2)) // countif arity
	assert.Equal(t, "60", cellAt(t, g, 1, 3))                   // sumif without sum range

	// sumif arity (one arg) and an error at a matching sum position propagate.
	assert.Equal(t, string(sheet.ErrValue), cellAt(t, compute(t, "1\t=sumif(A1)\n"), 0, 1))
	errSum := compute(t, "10\t20\t=1/0\t5\n=sumif(A1:B1, \">5\", C1:D1)\n") // C1 is #DIV/0!
	assert.Equal(t, string(sheet.ErrDiv), cellAt(t, errSum, 1, 0))
}
