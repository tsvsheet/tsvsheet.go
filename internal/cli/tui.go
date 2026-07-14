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
		Name:      cmdTUI,
		Usage:     "Edit a worksheet in a terminal UI.",
		ArgsUsage: "<template> <data>",
		Description: `Open the worksheet in a terminal spreadsheet: navigate the computed grid,
edit data cells and the template, recompute, and save — the same capabilities
as the browser editor, driven by the same engine. The template and data are
required positional file paths.

Examples:
  tsvsheet tui sheet.tsvt sheet.tsv`,
		Action: streamAction(func(s Streams, args positional) error {
			cfg.template = args.at(0)
			cfg.data = args.at(1)
			return runTUI(s, cfg)
		}),
	}
}
