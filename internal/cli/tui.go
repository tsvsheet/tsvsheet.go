package cli

import (
	"github.com/urfave/cli/v3"

	"github.com/uplang/tsvsheet.go/internal/sheet"
)

// tuiConfig binds the tui command's spreadsheet path, path-access mode,
// auto-refresh cadence (a duration or an isnow pattern; empty = auto), and the
// resource limits the editing session enforces.
type tuiConfig struct {
	source       sourcePath
	refresh      string
	isUnconfined pathAccess
	limits       sheet.Limits
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
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: flagAllowAnyPaths, Usage: usageAllowAnyPaths, Destination: &isUnconfined},
			&cli.StringFlag{
				Name:        flagRefreshInterval,
				Usage:       `Recompute the view: a duration (30s) or an isnow pattern ("M-F +[30mn]"); 0 disables. Default: 1s when the sheet has clock functions, else off`,
				Destination: &cfg.refresh,
			},
		},
		Action: limitedAction(func(s Streams, args positional, limits sheet.Limits) error {
			cfg.source = args.at(0)
			cfg.isUnconfined = pathAccess(isUnconfined)
			cfg.limits = limits
			return runTUI(s, cfg)
		}),
	}
}
