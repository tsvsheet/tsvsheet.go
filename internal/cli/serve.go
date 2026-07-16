package cli

import (
	"context"

	"github.com/uplang/go-tsvsheet"
	"github.com/urfave/cli/v3"

	"github.com/uplang/tsvsheet.go/internal/importer"
)

// serveConfig binds the serve command's spreadsheet path, bind address,
// path-access mode, auto-refresh cadence (a duration or an isnow pattern), the
// resource limits the editing session enforces, and the content-typed import
// fetcher (nil when imports are off) with its refresh cache.
type serveConfig struct {
	fetcher      tsvsheet.Fetcher
	cache        *importer.Cache
	source       sourcePath
	host         string
	refresh      string
	limits       tsvsheet.Limits
	port         int
	isUnconfined pathAccess
}

// defaultServeHost is the loopback address serve binds by default — a single-user
// local editor stays off the network unless the operator opts in.
const defaultServeHost = "127.0.0.1"

// flagRefreshInterval sets the browser's auto-refresh cadence for volatile
// (clock-dependent) cells.
const flagRefreshInterval = "refresh-interval"

// serveCommand builds the `serve` command.
func serveCommand() *cli.Command {
	isUnconfined := false
	cfg := serveConfig{}
	return &cli.Command{
		Name:      cmdServe,
		Usage:     "Serve a browser spreadsheet editor.",
		ArgsUsage: "<sheet>",
		Description: `Host a local web spreadsheet backed by the tsvsheet engine: edit any cell
(a value or an =formula) in the browser, recompute live, and save. The sheet
is a required positional file path (serve saves edits back to it, so stdin is
not accepted).

This is a single-user local editor: the browser reads and WRITES host files
(Save overwrites the sheet, and references can read its directory). It binds
127.0.0.1 by default and refuses cross-origin requests; do not bind a
non-loopback --host on an untrusted network.

Examples:
  tsvsheet serve sheet.tsvt
  tsvsheet serve --host 0.0.0.0 --port 8080 sheet.tsvt`,
		Flags: append([]cli.Flag{
			&cli.StringFlag{
				Name:        "host",
				Sources:     cli.EnvVars("HOST"),
				Value:       defaultServeHost,
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
			&cli.BoolFlag{Name: flagAllowAnyPaths, Usage: usageAllowAnyPaths, Destination: &isUnconfined},
			&cli.StringFlag{
				Name:        flagRefreshInterval,
				Usage:       `Auto-recompute the browser view: a duration (30s) or an isnow pattern ("M-F +[30mn] >=9 <=17"); 0 disables. Default: 1s when the sheet has clock functions (TODAY/NOW/ISNOW), else off`,
				Destination: &cfg.refresh,
			},
		}, importFlags()...),
		Action: func(ctx context.Context, c *cli.Command) error {
			fetcher, cache, err := resolveImport(c)
			if err != nil {
				return err
			}
			cfg.source = positional(c.Args().Slice()).at(0)
			cfg.isUnconfined = pathAccess(isUnconfined)
			cfg.limits = maxCellsLimits(c)
			cfg.fetcher, cfg.cache = fetcher, cache
			return runServe(ctx, cfg)
		},
	}
}
