package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/tsvsheet/go-tsvsheet"
	"github.com/urfave/cli/v3"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

// evalArg is the raw expression argument: the first positional, or "" to read
// the expression from stdin.
type evalArg string

// evalExpr is a resolved, non-empty expression ready to compute — the input
// with surrounding whitespace and a single leading '=' stripped.
type evalExpr string

// runEval computes a single formula/expression and writes its value to stdout,
// followed by a newline. The expression comes from arg, or from stdin when arg
// is empty (unix discipline). A leading '=' is optional. The expression is
// wrapped as the sole cell of a one-cell sheet and computed, so it may use any
// function or operator the engine supports; a reference to another cell has no
// surrounding grid to resolve against and yields #REF!, which is correct.
func runEval(streams Streams, arg evalArg, limits tsvsheet.Limits) error {
	expr, err := resolveExpr(streams.In, arg)
	if err != nil {
		return err
	}
	parsed, err := tsvsheet.Parse([]byte("=" + string(expr) + "\n"))
	if err != nil {
		return err
	}
	grid := parsed.ComputeWith(tsvsheet.ComputeOptions{At: time.Now(), Limits: limits})
	if _, err := fmt.Fprintln(streams.Out, grid[0][0]); err != nil {
		return tsvsheet.ErrWriteFile.With(err)
	}
	return nil
}

// resolveExpr resolves the expression to compute: arg when non-empty, otherwise
// the whole of stdin. Surrounding whitespace and a single leading '=' are
// stripped; an expression that is empty after stripping is ErrMissingArgument.
func resolveExpr(in io.Reader, arg evalArg) (evalExpr, error) {
	raw := string(arg)
	if raw == "" {
		data, err := readAll(in)
		if err != nil {
			return "", err
		}
		raw = string(data)
	}
	expr := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "="))
	if expr == "" {
		return "", constants.ErrMissingArgument.With(nil, "argument", "expression")
	}
	if strings.ContainsAny(expr, "\t\n") {
		return "", constants.ErrMultiCellExpression.With(nil)
	}
	return evalExpr(expr), nil
}

// limitedAction adapts a positional-args + stream-injected function that needs
// the --max-cells resource limits (but no import fetcher or sheet loader) to a
// cli Action — the shape eval uses, since a bare expression has no cross-sheet
// or import references.
func limitedAction(fn func(Streams, positional, tsvsheet.Limits) error) cli.ActionFunc {
	return func(_ context.Context, c *cli.Command) error {
		streams := Streams{In: stdin, Out: c.Root().Writer, Err: stderr}
		return fn(streams, positional(c.Args().Slice()), maxCellsLimits(c))
	}
}

// evalCommand builds the `eval` command.
func evalCommand() *cli.Command {
	return &cli.Command{
		Name:      cmdEval,
		Usage:     "Compute a single formula or expression and print its value.",
		ArgsUsage: "[expression]",
		Description: `Evaluate one formula or expression and write its value to stdout. The
expression is positional; omitted (or empty) reads it from stdin. A leading
'=' is optional. There is no surrounding grid, so a reference to another cell
(A2, B3) resolves to #REF! — eval is for self-contained expressions.

Examples:
  tsv eval '=1+2'          # 3
  tsv eval 'SUM(1,2,3)'    # 6
  tsv eval '1/0'           # #DIV/0!
  echo '=2^10' | tsv eval  # 1024`,
		Action: limitedAction(func(s Streams, args positional, limits tsvsheet.Limits) error {
			return runEval(s, evalArg(args.text(0)), limits)
		}),
	}
}
