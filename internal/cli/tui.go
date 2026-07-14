package cli

import (
	"github.com/urfave/cli/v3"
)

// tuiConfig binds the tui command's worksheet paths.
type tuiConfig struct {
	template sourcePath
	data     sourcePath
}

// tuiCommand builds the `tui` command.
func tuiCommand() *cli.Command {
	cfg := tuiConfig{}
	return &cli.Command{
		Name:      "tui",
		Usage:     "Edit a worksheet in a terminal UI.",
		ArgsUsage: " ",
		Description: `Open the worksheet in a terminal spreadsheet: navigate the computed grid,
edit data cells and the template, recompute, and save — the same capabilities
as the browser editor, driven by the same engine.

Examples:
  tsvsheet tui --template sheet.tsvt --data sheet.tsv`,
		Flags:  sourceFlags(&cfg.template, &cfg.data),
		Action: streamAction(func(s Streams) error { return runTUI(s, cfg) }),
	}
}
