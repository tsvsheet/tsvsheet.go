package cli

import (
	"github.com/urfave/cli/v3"

	"github.com/uplang/tsvsheet.go/internal/sheet"
)

// cellView is the JSON projection of one non-empty cell: its A1 address, source
// text, whether it is a formula, and — with --value — its computed value.
type cellView struct {
	Value     *string `json:"value,omitempty"`
	Cell      string  `json:"cell"`
	Source    string  `json:"source"`
	IsFormula bool    `json:"formula"`
}

// sheetView is the JSON projection of a parsed spreadsheet.
type sheetView struct {
	Cells []cellView `json:"cells"`
}

// valueOutput requests each cell's computed value in the JSON output (the
// --value flag).
type valueOutput bool

// runParse parses a spreadsheet and writes its non-empty cells as JSON — a
// stable, jq-friendly surface for scripting. When isValues is set, each cell
// also carries its computed value (the sheet is evaluated, resolving embeds).
func runParse(streams Streams, source sourcePath, isValues valueOutput) error {
	reader, release, err := source.open(streams.In)
	if err != nil {
		return err
	}
	defer func() { _ = release() }()

	parsed, err := parseSheet(reader)
	if err != nil {
		return err
	}
	var values sheet.Grid
	if isValues {
		values = parsed.ComputeWith(computeOptions(source))
	}
	return writeJSON(streams.Out, sheetView{Cells: cellViews(parsed, values)})
}

// cellViews projects every non-empty cell to its JSON view, attaching the
// computed value from values when it is non-nil (the --value flag).
func cellViews(s sheet.Sheet, values sheet.Grid) []cellView {
	cells := s.Cells()
	views := make([]cellView, len(cells))
	for i, c := range cells {
		view := cellView{Cell: c.Address.String(), Source: c.Text, IsFormula: c.IsFormula}
		if values != nil {
			computed := values[c.Address.Row][c.Address.Col]
			view.Value = &computed
		}
		views[i] = view
	}
	return views
}

// parseCommand builds the `parse` command.
func parseCommand() *cli.Command {
	isValues := false
	return &cli.Command{
		Name:      cmdParse,
		Usage:     "Parse a spreadsheet and emit its cells as JSON.",
		ArgsUsage: argSheetOptional,
		Description: `Parse a .tsvt spreadsheet (positional; omitted or "-" reads stdin) and write
its non-empty cells — address, source, and whether each is a formula — as JSON
to stdout. With --value, each cell also carries its computed value.

Examples:
  tsvsheet parse sheet.tsvt | jq '.cells[] | select(.formula)'
  tsvsheet parse --value sheet.tsvt | jq '.cells[] | {cell, value}'
  cat sheet.tsvt | tsvsheet parse`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "value",
				Usage:       "Include each cell's computed value",
				Destination: &isValues,
			},
		},
		Action: streamAction(func(s Streams, args positional) error {
			return runParse(s, args.at(0), valueOutput(isValues))
		}),
	}
}
