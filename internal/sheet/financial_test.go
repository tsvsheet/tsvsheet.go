package sheet_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uplang/tsvsheet.go/internal/sheet"
)

func TestFinancial_Values(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"round(pmt(0.05 / 12, 60, 20000), 2)":  "-377.42",
		"pmt(0, 10, 1000)":                     "-100", // zero rate
		"round(fv(0.05 / 12, 60, -377.42), 2)": "25666.86",
		"fv(0, 5, -100)":                       "500", // zero rate
		"round(pv(0.05, 10, -1000), 2)":        "7721.73",
		"pv(0, 5, -100)":                       "500", // zero rate
		"round(npv(0.1, 100, 200, 300), 2)":    "481.59",
		"sln(10000, 1000, 5)":                  "1800",
	}
	for expr, want := range cases {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, formula1(t, expr))
		})
	}
}

func TestFinancial_Errors(t *testing.T) {
	t.Parallel()

	assert.Equal(t, string(sheet.ErrDiv), formula1(t, "sln(1000, 0, 0)")) // zero life

	// A1 is text; a non-numeric argument in any position propagates #VALUE!.
	for _, expr := range []string{
		"=pmt(A1, 10, 1000)", "=pmt(0.05, 10, 1000, A1)", "=fv(A1, 5, -100)",
		"=pv(A1, 5, -100)", "=npv(A1, 100)", "=npv(0.1, A1)", "=sln(A1, 0, 5)",
	} {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, "#VALUE!", cellAt(t, compute(t, "hi\t"+expr+"\n"), 0, 1))
		})
	}
}
