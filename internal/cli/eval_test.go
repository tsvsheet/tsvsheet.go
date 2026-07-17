package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uplang/go-tsvsheet"

	"github.com/uplang/tsvsheet.go/internal/constants"
)

// TestRunEval covers the value-producing paths: a numeric expression, a
// function call, an error-value result, the optional leading '=', a bare cell
// reference resolving to #REF! (no surrounding grid), and stdin input — from
// both the arg and stdin sources.
func TestRunEval(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		arg  string
		in   string
		want string
	}{
		{name: "numeric with equals", arg: "=1+2", want: "3\n"},
		{name: "numeric without equals", arg: "1+2", want: "3\n"},
		{name: "function call", arg: "SUM(1,2,3)", want: "6\n"},
		{name: "divide by zero", arg: "1/0", want: "#DIV/0!\n"},
		{name: "cell reference out of grid", arg: "A2", want: "#REF!\n"},
		{name: "surrounding whitespace", arg: "  =2^10  ", want: "1024\n"},
		{name: "from stdin", arg: "", in: "=SUM(4,5)\n", want: "9\n"},
		{name: "from stdin without equals", arg: "", in: "3*3\n", want: "9\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			streams, out, _ := streamsWith(tc.in)
			require.NoError(t, runEval(streams, evalArg(tc.arg), tsvsheet.DefaultLimits()))
			assert.Equal(t, tc.want, out.String())
		})
	}
}

// TestRunEval_EmptyInput proves an empty expression (from the arg or from
// stdin, before or after the leading '=' is stripped) is the specific
// ErrMissingArgument sentinel — not a syntax error and not a crash.
func TestRunEval_EmptyInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		arg  string
		in   string
	}{
		{name: "no arg, empty stdin", arg: "", in: ""},
		{name: "no arg, whitespace stdin", arg: "", in: "  \n"},
		{name: "arg is only an equals", arg: "=", in: ""},
		{name: "arg is only whitespace", arg: "   ", in: "should not be read"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			streams, _, _ := streamsWith(tc.in)
			err := runEval(streams, evalArg(tc.arg), tsvsheet.DefaultLimits())
			require.Error(t, err)
			assert.ErrorIs(t, err, constants.ErrMissingArgument)
		})
	}
}

// TestRunEval_SyntaxError proves a malformed expression surfaces as ErrSyntax
// (a clear error, not a panic).
func TestRunEval_SyntaxError(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("")
	err := runEval(streams, evalArg("1+"), tsvsheet.DefaultLimits())
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrSyntax)
}

// TestRunEval_MultiCellRejected proves an expression carrying a TAB or newline
// (the grid's own separators) is rejected, not silently truncated to its first
// cell.
func TestRunEval_MultiCellRejected(t *testing.T) {
	t.Parallel()

	for _, arg := range []string{"1+1\tSUM(9,9)", "1+1\n=99"} {
		t.Run(arg, func(t *testing.T) {
			t.Parallel()
			streams, _, _ := streamsWith("")
			err := runEval(streams, evalArg(arg), tsvsheet.DefaultLimits())
			require.Error(t, err)
			assert.ErrorIs(t, err, constants.ErrMultiCellExpression)
		})
	}
}

// TestRunEval_ReadError proves a stdin read failure is surfaced.
func TestRunEval_ReadError(t *testing.T) {
	t.Parallel()

	streams := Streams{In: failReader{}, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := runEval(streams, "", tsvsheet.DefaultLimits())
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrReadInput)
}

// TestRunEval_WriteError proves an output write failure is surfaced as
// ErrWriteFile.
func TestRunEval_WriteError(t *testing.T) {
	t.Parallel()

	streams := Streams{In: strings.NewReader(""), Out: failWriter{}, Err: &bytes.Buffer{}}
	err := runEval(streams, evalArg("1+2"), tsvsheet.DefaultLimits())
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrWriteFile)
}

// TestCLI_Eval proves the command is wired: it dispatches the positional
// expression through the run function to stdout.
func TestCLI_Eval(t *testing.T) {
	out, err := runCLI(t, "eval", "=1+2")
	require.NoError(t, err)
	assert.Equal(t, "3\n", out)
}

// TestCLI_EvalStdin proves the command reads the expression from stdin when no
// positional is given.
func TestCLI_EvalStdin(t *testing.T) {
	withStdin(t, "=SUM(1,2,3)\n")
	out, err := runCLI(t, "eval")
	require.NoError(t, err)
	assert.Equal(t, "6\n", out)
}

// TestCLI_EvalMaxCells proves the command honors the global --max-cells cap
// threaded into the compute pass.
func TestCLI_EvalMaxCells(t *testing.T) {
	out, err := runCLI(t, "--max-cells", "5", "eval", "=SEQUENCE(10)")
	require.NoError(t, err)
	assert.Contains(t, out, "#VALUE!") // 10 cells exceeds the 5-cell cap
}
