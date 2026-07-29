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
func runEval(streams Streams, arg evalArg, limits tsvsheet.Limits, fetcher tsvsheet.Fetcher) error {
	expr, err := resolveExpr(streams.In, arg)
	if err != nil {
		return err
	}
	parsed, err := tsvsheet.Parse([]byte("=" + string(expr) + "\n"))
	if err != nil {
		return err
	}
	grid := parsed.ComputeWith(tsvsheet.ComputeOptions{At: time.Now(), Limits: limits, Fetcher: fetcher})
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

IMPORT* works here as it does anywhere: an absolute URL needs --allow-import
and an --import-host, and a relative reference resolves against --data.

Examples:
  tsv eval '=1+2'          # 3
  tsv eval 'SUM(1,2,3)'    # 6
  tsv eval '1/0'           # #DIV/0!
  echo '=2^10' | tsv eval  # 1024
  tsv eval --data ./data '=importcell("rate.tsv")'`,
		Flags: append(importFlags(), dataFlags()...),
		Action: func(_ context.Context, c *cli.Command) error {
			base, closeData, err := resolveData(c)
			if err != nil {
				return err
			}
			defer func() { _ = closeData() }()
			fetcher, _, err := resolveImport(c, base)
			if err != nil {
				return err
			}
			streams := Streams{In: stdin, Out: c.Root().Writer, Err: stderr}
			return withUsageHelp(c,
				runEval(streams, evalArg(positional(c.Args().Slice()).text(0)), maxCellsLimits(c), fetcher))
		},
	}
}
