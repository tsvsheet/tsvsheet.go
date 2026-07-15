package cli

import (
	"encoding/json"

	"github.com/urfave/cli/v3"

	"github.com/uplang/tsvsheet.go/internal/constants"
	"github.com/uplang/tsvsheet.go/internal/sheet"
)

// sheetView is the JSON projection of a spreadsheet: the source grid (rows) and
// — with --value — the computed grid (values). Both are 2-D row-major arrays, so
// a .tsvt round-trips through JSON (rows are lossless) and the grid is clean to
// munge with jq (e.g. `.rows[1][3]`, `.values`).
type sheetView struct {
	Rows   sheet.Grid `json:"rows"`
	Values sheet.Grid `json:"values,omitempty"`
}

// valueOutput requests the computed grid in the JSON output (the --value flag).
type valueOutput bool

// runParse parses a spreadsheet and writes its source grid as JSON — a stable,
// jq-friendly, round-trippable surface (see from-json). With isValues, the
// computed grid is included too (the sheet is evaluated, resolving embeds).
func runParse(streams Streams, source sourcePath, isValues valueOutput, isUnconfined pathAccess) error {
	reader, release, err := source.open(streams.In)
	if err != nil {
		return err
	}
	defer func() { _ = release() }()

	parsed, err := parseSheet(reader)
	if err != nil {
		return err
	}
	view := sheetView{Rows: parsed.Source()}
	if isValues {
		view.Values = parsed.ComputeWith(computeOptions(source, isUnconfined))
	}
	return writeJSON(streams.Out, view)
}

// runFromJSON reads a sheetView JSON (from parse) and writes its source rows as
// TSV — the inverse of parse, so a spreadsheet round-trips through JSON. Any
// computed values in the input are ignored; the source rows are authoritative.
func runFromJSON(streams Streams, source sourcePath) error {
	reader, release, err := source.open(streams.In)
	if err != nil {
		return err
	}
	defer func() { _ = release() }()

	var view sheetView
	if err := json.NewDecoder(reader).Decode(&view); err != nil {
		return constants.ErrSyntax.With(err)
	}
	return sheet.WriteTSV(streams.Out, view.Rows)
}

// parseCommand builds the `parse` command.
func parseCommand() *cli.Command {
	isValues := false
	isUnconfined := false
	return &cli.Command{
		Name:      cmdParse,
		Usage:     "Parse a spreadsheet and emit its grid as JSON.",
		ArgsUsage: argSheetOptional,
		Description: `Parse a .tsvt spreadsheet (positional; omitted or "-" reads stdin) and write
its source grid as JSON {"rows": [[...]]} to stdout — round-trippable via
from-json and clean to munge with jq. With --value, the computed grid is
included as "values".

Examples:
  tsvsheet parse sheet.tsvt | jq '.rows[1]'
  tsvsheet parse --value sheet.tsvt | jq '.values'
  tsvsheet parse sheet.tsvt | tsvsheet from-json   # round-trip
  cat sheet.tsvt | tsvsheet parse`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "value",
				Usage:       "Include the computed grid as \"values\"",
				Destination: &isValues,
			},
			&cli.BoolFlag{Name: flagAllowAnyPaths, Usage: usageAllowAnyPaths, Destination: &isUnconfined},
		},
		Action: streamAction(func(s Streams, args positional) error {
			return runParse(s, args.at(0), valueOutput(isValues), pathAccess(isUnconfined))
		}),
	}
}

// fromJSONCommand builds the `from-json` command.
func fromJSONCommand() *cli.Command {
	return &cli.Command{
		Name:      cmdFromJSON,
		Usage:     "Reconstruct a spreadsheet from parse's JSON.",
		ArgsUsage: argSheetOptional,
		Description: `Read a {"rows": [[...]]} JSON document (as emitted by parse; positional,
omitted or "-" reads stdin) and write the spreadsheet back as TSV to stdout —
the inverse of parse. Computed "values" in the input are ignored.

Examples:
  tsvsheet parse sheet.tsvt | tsvsheet from-json
  jq '.rows[0] |= ascii_upcase' data.json | tsvsheet from-json`,
		Action: streamAction(func(s Streams, args positional) error {
			return runFromJSON(s, args.at(0))
		}),
	}
}
