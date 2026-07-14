package cli

import (
	"context"

	"github.com/urfave/cli/v3"
)

// serveConfig binds the serve command's worksheet paths and bind address.
type serveConfig struct {
	template sourcePath
	data     sourcePath
	host     string
	port     int
}

// serveCommand builds the `serve` command.
func serveCommand() *cli.Command {
	cfg := serveConfig{}
	return &cli.Command{
		Name:      cmdServe,
		Usage:     "Serve a browser spreadsheet editor for a worksheet.",
		ArgsUsage: "<template> <data>",
		Description: `Host a local web spreadsheet backed by the tsvsheet engine: edit data cells
and the template in the browser, recompute live, and save both files. The
template and data are required positional file paths (serve saves edits back
to them, so stdin is not accepted).

Examples:
  tsvsheet serve sheet.tsvt sheet.tsv
  tsvsheet serve --host 0.0.0.0 --port 8080 sheet.tsvt sheet.tsv`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "host",
				Sources:     cli.EnvVars("HOST"),
				Value:       "127.0.0.1",
				Usage:       "Host address to bind",
				Destination: &cfg.host,
			},
			&cli.IntFlag{
				Name:        "port",
				Aliases:     []string{"p"},
				Sources:     cli.EnvVars("PORT"),
				Value:       8080,
				Usage:       "Port to listen on",
				Destination: &cfg.port,
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			args := positional(c.Args().Slice())
			cfg.template = args.at(0)
			cfg.data = args.at(1)
			return runServe(ctx, cfg)
		},
	}
}
