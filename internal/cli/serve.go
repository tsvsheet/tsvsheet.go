package cli

import (
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
		Name:      "serve",
		Usage:     "Serve a browser spreadsheet editor for a worksheet.",
		ArgsUsage: " ",
		Description: `Host a local web spreadsheet backed by the tsvsheet engine: edit data cells
and the template in the browser, recompute live, and save both files.

Examples:
  tsvsheet serve --template sheet.tsvt --data sheet.tsv
  tsvsheet serve --host 0.0.0.0 --port 8080 -t sheet.tsvt -d sheet.tsv`,
		Flags: append(
			sourceFlags(&cfg.template, &cfg.data),
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
		),
		Action: streamAction(func(s Streams) error { return runServe(s, cfg) }),
	}
}
