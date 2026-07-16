package cli

import (
	"context"

	"github.com/uplang/go-tsvsheet"
	"github.com/urfave/cli/v3"

	"github.com/uplang/tsvsheet.go/internal/importer"
)

// tuiConfig binds the tui command's spreadsheet path, path-access mode,
// auto-refresh cadence (a duration or an isnow pattern; empty = auto), the
// resource limits the editing session enforces, and the content-typed import
// fetcher (nil when imports are off) with its refresh cache.
type tuiConfig struct {
	fetcher      tsvsheet.Fetcher
	cache        *importer.Cache
	source       sourcePath
	refresh      string
	limits       tsvsheet.Limits
	isUnconfined pathAccess
}

// tuiCommand builds the `tui` command.
func tuiCommand() *cli.Command {
	isUnconfined := false
	cfg := tuiConfig{}
	return &cli.Command{
		Name:      cmdTUI,
		Usage:     "Edit a spreadsheet in a terminal UI.",
		ArgsUsage: "<sheet>",
		Description: `Open the spreadsheet in a terminal grid: navigate cells, edit any cell (a
value or an =formula), recompute, and save — the same capabilities as the
browser editor, driven by the same engine. The sheet is a required positional
file path.

Examples:
  tsvsheet tui sheet.tsvt`,
		Flags: append([]cli.Flag{
			&cli.BoolFlag{Name: flagAllowAnyPaths, Usage: usageAllowAnyPaths, Destination: &isUnconfined},
			&cli.StringFlag{
				Name:        flagRefreshInterval,
				Usage:       `Recompute the view: a duration (30s) or an isnow pattern ("M-F +[30mn]"); 0 disables. Default: 1s when the sheet has clock functions, else off`,
				Destination: &cfg.refresh,
			},
		}, importFlags()...),
		Action: func(_ context.Context, c *cli.Command) error {
			fetcher, cache, err := resolveImport(c)
			if err != nil {
				return err
			}
			streams := Streams{In: stdin, Out: c.Root().Writer, Err: stderr}
			cfg.source = positional(c.Args().Slice()).at(0)
			cfg.isUnconfined = pathAccess(isUnconfined)
			cfg.limits = maxCellsLimits(c)
			cfg.fetcher, cfg.cache = fetcher, cache
			return runTUI(streams, cfg)
		},
	}
}
