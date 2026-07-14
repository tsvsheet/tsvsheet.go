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

const (
	templateFlag = "template"
	dataFlag     = "data"
	jsonFlag     = "json"
	cellFlag     = "cell"
)

// sourceFlags builds the shared --template/--data path flags bound to the given
// source paths.
func sourceFlags(template, data *sourcePath) []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        templateFlag,
			Aliases:     []string{"t"},
			Sources:     cli.EnvVars("TSVSHEET_TEMPLATE"),
			Usage:       "Template .tsvt path ('-' or omitted = stdin)",
			Destination: (*string)(template),
		},
		&cli.StringFlag{
			Name:        dataFlag,
			Aliases:     []string{"d"},
			Sources:     cli.EnvVars("TSVSHEET_DATA"),
			Usage:       "Data .tsv path ('-' or omitted = stdin)",
			Destination: (*string)(data),
		},
	}
}

// templateFlagOnly builds just the --template flag for commands that need no
// data grid (parse, check).
func templateFlagOnly(template *sourcePath) []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        templateFlag,
			Aliases:     []string{"t"},
			Sources:     cli.EnvVars("TSVSHEET_TEMPLATE"),
			Usage:       "Template .tsvt path ('-' or omitted = stdin)",
			Destination: (*string)(template),
		},
	}
}

// streamAction adapts a stream-injected function to a cli Action, wiring stdout
// from the command writer and stderr from the indirected stream.
func streamAction(fn func(Streams) error) cli.ActionFunc {
	return func(_ context.Context, c *cli.Command) error {
		return fn(Streams{In: stdin, Out: c.Root().Writer, Err: stderr})
	}
}
