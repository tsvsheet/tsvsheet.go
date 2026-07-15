package tsvt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/constants"
	"github.com/uplang/tsvsheet.go/internal/tsvt"
)

// parse builds the AST of a formula, failing the test on a syntax error.
func parse(t *testing.T, src string) tsvt.Expr {
	t.Helper()
	e, err := tsvt.ParseFormula(tsvt.FormulaText(src))
	require.NoError(t, err)
	return e
}

func TestBuild_Number(t *testing.T) {
	t.Parallel()
	assert.Equal(t, tsvt.Number{Text: "42"}, parse(t, "42"))
	assert.Equal(t, tsvt.Number{Text: "3.14"}, parse(t, "3.14"))
}

func TestBuild_String(t *testing.T) {
	t.Parallel()
	assert.Equal(t, tsvt.StringLit{Value: "hi"}, parse(t, `"hi"`))
}

func TestBuild_Bool(t *testing.T) {
	t.Parallel()
	assert.Equal(t, tsvt.BoolLit{IsTrue: true}, parse(t, "TRUE"))
	assert.Equal(t, tsvt.BoolLit{IsTrue: false}, parse(t, "FALSE"))
}

func TestBuild_Error(t *testing.T) {
	t.Parallel()
	assert.Equal(t, tsvt.ErrorLit{Code: "#N/A"}, parse(t, "#N/A"))
	assert.Equal(t, tsvt.ErrorLit{Code: "#REF!"}, parse(t, "#REF!"))
}

func TestBuild_Reference(t *testing.T) {
	t.Parallel()
	want := tsvt.RefOperand{Ref: tsvt.RangeRef{From: tsvt.CellRef{Col: "B", Row: 2}}}
	assert.Equal(t, want, parse(t, "B2"))
	// $-absolute markers are accepted and carry no positional difference.
	assert.Equal(t, want, parse(t, "$B$2"))
	assert.Equal(t, want, parse(t, "B$2"))
}

func TestBuild_Range(t *testing.T) {
	t.Parallel()
	to := tsvt.CellRef{Col: "C", Row: 3}
	want := tsvt.RefOperand{Ref: tsvt.RangeRef{From: tsvt.CellRef{Col: "A", Row: 1}, To: &to}}
	assert.Equal(t, want, parse(t, "A1:C3"))
}

func TestBuild_Unary(t *testing.T) {
	t.Parallel()
	assert.Equal(t, tsvt.Unary{X: tsvt.Number{Text: "5"}, Op: tsvt.OpNeg}, parse(t, "-5"))
	assert.Equal(t, tsvt.Unary{X: tsvt.Number{Text: "5"}, Op: tsvt.OpPos}, parse(t, "+5"))
}

func TestBuild_Percent(t *testing.T) {
	t.Parallel()
	assert.Equal(t, tsvt.Percent{X: tsvt.Number{Text: "50"}}, parse(t, "50%"))
}

func TestBuild_BinaryOperators(t *testing.T) {
	t.Parallel()
	cases := map[string]tsvt.BinaryOp{
		"2 ^ 8":     tsvt.OpPow,
		"1 * 2":     tsvt.OpMul,
		"1 / 2":     tsvt.OpDiv,
		"1 + 2":     tsvt.OpAdd,
		"1 - 2":     tsvt.OpSub,
		`"a" & "b"`: tsvt.OpCat,
		"1 = 2":     tsvt.OpEq,
		"1 <> 2":    tsvt.OpNe,
		"1 < 2":     tsvt.OpLt,
		"1 <= 2":    tsvt.OpLe,
		"1 > 2":     tsvt.OpGt,
		"1 >= 2":    tsvt.OpGe,
	}
	for src, op := range cases {
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, op, parse(t, src).(tsvt.Binary).Op)
		})
	}
}

func TestBuild_Precedence(t *testing.T) {
	t.Parallel()
	// (1 + 2) * 3 groups the addition first.
	outer := parse(t, "(1 + 2) * 3").(tsvt.Binary)
	assert.Equal(t, tsvt.OpMul, outer.Op)
	assert.Equal(t, tsvt.OpAdd, outer.Left.(tsvt.Binary).Op)
}

func TestBuild_Call(t *testing.T) {
	t.Parallel()
	multi := parse(t, "sum(A1, B1)").(tsvt.Call)
	assert.Equal(t, "sum", multi.Name)
	assert.Len(t, multi.Args, 2)

	assert.Equal(t, "IF", parse(t, "IF(1, 2, 3)").(tsvt.Call).Name) // name via COL
	assert.Empty(t, parse(t, "now()").(tsvt.Call).Args)             // no arguments
}

func TestBuild_FractionalRowRejected(t *testing.T) {
	t.Parallel()
	// A fractional A1 row is a syntax error; assert it surfaces through every
	// builder path that can contain a reference.
	for _, src := range []string{"B2.5", "-B2.5", "B2.5%", "B2.5 + 1", "1 + B2.5", "sum(B2.5)", "A1:C3.5"} {
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			_, err := tsvt.ParseFormula(tsvt.FormulaText(src))
			require.Error(t, err)
			assert.ErrorIs(t, err, constants.ErrSyntax)
		})
	}
}
