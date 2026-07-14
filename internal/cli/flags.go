package cli

import (
	"context"
	"io"
	"os"

	"github.com/urfave/cli/v3"
)

// stdin is indirected so tests can substitute an input stream.
var stdin io.Reader = os.Stdin

// stderr is indirected so tests can capture diagnostics.
var stderr io.Writer = os.Stderr

// jsonFlag names the explain command's --json option.
const jsonFlag = "json"

// positional is a command's positional arguments. Required inputs (template,
// data, cell) are positional — never flags — so invocations read as
// `tsvsheet serve sheet.tsvt sheet.tsv`.
type positional []string

// at returns the i-th positional argument as a source path, or "" (meaning
// stdin) when the argument is absent.
func (p positional) at(i int) sourcePath {
	if i < len(p) {
		return sourcePath(p[i])
	}
	return ""
}

// text returns the i-th positional argument verbatim, or "" when absent.
func (p positional) text(i int) string {
	if i < len(p) {
		return p[i]
	}
	return ""
}

// streamAction adapts a positional-args + stream-injected function to a cli
// Action, wiring stdout from the command writer and stderr from the indirected
// stream, and the positional arguments from the parsed command line.
func streamAction(fn func(Streams, positional) error) cli.ActionFunc {
	return func(_ context.Context, c *cli.Command) error {
		streams := Streams{In: stdin, Out: c.Root().Writer, Err: stderr}
		return fn(streams, positional(c.Args().Slice()))
	}
}
