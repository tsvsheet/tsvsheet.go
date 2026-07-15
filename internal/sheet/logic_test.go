package sheet_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uplang/tsvsheet.go/internal/sheet"
)

func TestLogic_BooleansAndConditionals(t *testing.T) {
	t.Parallel()

	// A1=2, C1=3, D1=4 (B1 holds the formula).
	cases := map[string]string{
		"true()":  "TRUE",
		"false()": "FALSE",
		"na()":    string(sheet.ErrNA),

		"and(TRUE, 1, 5)": "TRUE",
		"and(TRUE, 0)":    "FALSE",
		"or(FALSE, 0)":    "FALSE",
		"or(0, 7)":        "TRUE",
		"not(FALSE)":      "TRUE",
		"not(1)":          "FALSE",
		"xor(1, 1, 1)":    "TRUE",
		"xor(1, 1)":       "FALSE",

		"ifs(FALSE, 1, TRUE, 2)":   "2",
		"ifs(A1 > C1, 1, TRUE, 9)": "9",

		"iferror(D1 / 0, 7)": "7",
		"iferror(D1, 7)":     "4",
		"ifna(na(), 5)":      "5",
		"ifna(D1 / 0, 5)":    string(sheet.ErrDiv), // IFNA does not catch #DIV/0!
		"ifna(D1, 5)":        "4",

		"switch(A1, 1, 10, 2, 20, 99)": "20", // matches case 2
		"switch(A1, 1, 10, 99)":        "99", // default
	}
	for expr, want := range cases {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, formula1(t, expr))
		})
	}
}

func TestLogic_ConditionalArityAndErrors(t *testing.T) {
	t.Parallel()

	assert.Equal(t, string(sheet.ErrValue), formula1(t, "ifs(TRUE)"))          // odd count
	assert.Equal(t, string(sheet.ErrNA), formula1(t, "ifs(FALSE, 1)"))         // no truthy
	assert.Equal(t, string(sheet.ErrRef), formula1(t, "ifs(Z99, 1)"))          // error condition
	assert.Equal(t, string(sheet.ErrValue), formula1(t, "iferror(1)"))         // wrong arity
	assert.Equal(t, string(sheet.ErrValue), formula1(t, "switch(1, 2)"))       // too few
	assert.Equal(t, string(sheet.ErrRef), formula1(t, "switch(Z99, 1, 2)"))    // error subject
	assert.Equal(t, string(sheet.ErrNA), formula1(t, "switch(1, 2, 3, 4, 5)")) // no match, no default
	assert.Equal(t, string(sheet.ErrValue), formula1(t, "isnumber(1, 2)"))     // inspector arity
}

func TestLogic_Parity(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "TRUE", formula1(t, "iseven(4)"))
	assert.Equal(t, "FALSE", formula1(t, "iseven(3)"))
	assert.Equal(t, "TRUE", formula1(t, "isodd(-3)")) // negative parity
	assert.Equal(t, "FALSE", formula1(t, "isodd(4)"))
	assert.Equal(t, string(sheet.ErrRef), formula1(t, "iseven(Z99)"))            // error propagates
	assert.Equal(t, "#VALUE!", cellAt(t, compute(t, "hi\t=iseven(A1)\n"), 0, 1)) // non-number
}

func TestLogic_Inspectors(t *testing.T) {
	t.Parallel()

	// Each inspector observes the value's kind (errors and empties included).
	assert.Equal(t, "TRUE", cellAt(t, compute(t, "\t=isblank(A1)\n"), 0, 1))  // empty
	assert.Equal(t, "TRUE", cellAt(t, compute(t, "hi\t=istext(A1)\n"), 0, 1)) // text
	assert.Equal(t, "FALSE", cellAt(t, compute(t, "hi\t=isnumber(A1)\n"), 0, 1))
	assert.Equal(t, "FALSE", cellAt(t, compute(t, "hi\t=isnontext(A1)\n"), 0, 1))
	assert.Equal(t, "TRUE", formula1(t, "isnumber(A1)"))
	assert.Equal(t, "TRUE", formula1(t, "islogical(TRUE)"))
	assert.Equal(t, "TRUE", formula1(t, "iserror(D1 / 0)"))
	assert.Equal(t, "TRUE", formula1(t, "iserr(D1 / 0)")) // #DIV/0! is an err
	assert.Equal(t, "FALSE", formula1(t, "iserr(na())"))  // but #N/A is not
	assert.Equal(t, "TRUE", formula1(t, "isna(na())"))
	assert.Equal(t, "FALSE", formula1(t, "isna(D1 / 0)"))

	// N coerces; TYPE classifies.
	assert.Equal(t, "4", formula1(t, "n(D1)"))
	assert.Equal(t, "1", formula1(t, "n(TRUE)"))
	assert.Equal(t, "0", cellAt(t, compute(t, "hi\t=n(A1)\n"), 0, 1)) // text → 0
	assert.Equal(t, string(sheet.ErrRef), formula1(t, "n(Z99)"))      // error propagates
	assert.Equal(t, "1", formula1(t, "type(D1)"))
	assert.Equal(t, "2", cellAt(t, compute(t, "hi\t=type(A1)\n"), 0, 1))
	assert.Equal(t, "4", formula1(t, "type(TRUE)"))
	assert.Equal(t, "16", formula1(t, "type(D1 / 0)"))
}
