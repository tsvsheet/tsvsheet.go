package cli

import (
	"github.com/urfave/cli/v3"
)

// tuiConfig binds the tui command's spreadsheet path, path-access mode, and
// auto-refresh cadence (a duration or an isnow pattern; empty = auto).
type tuiConfig struct {
	source       sourcePath
	refresh      string
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
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: flagAllowAnyPaths, Usage: usageAllowAnyPaths, Destination: &isUnconfined},
			&cli.StringFlag{
				Name:        flagRefreshInterval,
				Usage:       `Recompute the view: a duration (30s) or an isnow pattern ("M-F +[30mn]"); 0 disables. Default: 1s when the sheet has clock functions, else off`,
				Destination: &cfg.refresh,
			},
		},
		Action: streamAction(func(s Streams, args positional) error {
			cfg.source = args.at(0)
			cfg.isUnconfined = pathAccess(isUnconfined)
			return runTUI(s, cfg)
		}),
	}
}
