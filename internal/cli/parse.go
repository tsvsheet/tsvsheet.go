package cli

import (
	"context"
	"encoding/json"

	"github.com/tsvsheet/go-tsvsheet"
	"github.com/urfave/cli/v3"
)

// sheetView is the JSON projection of a spreadsheet: the source grid (rows) and
// — with --value — the computed grid (values). Both are 2-D row-major arrays, so
// a .tsvt round-trips through JSON (rows are lossless) and the grid is clean to
// munge with jq (e.g. `.rows[1][3]`, `.values`).
type sheetView struct {
	Rows   tsvsheet.Grid `json:"rows"`
	Values tsvsheet.Grid `json:"values,omitempty"`
}

// valueOutput requests the computed grid in the JSON output (the --value flag).
type valueOutput bool

// runParse parses a spreadsheet and writes its source grid as JSON — a stable,
// jq-friendly, round-trippable surface (see from-json). With isValues, the
// computed grid is included too (the sheet is evaluated, resolving embeds).
func runParse(
	streams Streams,
	source sourcePath,
	isValues valueOutput,
	isUnconfined pathAccess,
	limits tsvsheet.Limits,
	fetcher tsvsheet.Fetcher,
) error {
	reader, release, err := source.open(streams.In)
	if err != nil {
		return err
	}
	defer func() { _ = release() }()

	parsed, err := parseSheet(reader, limits)
	if err != nil {
		return err
	}
	view := sheetView{Rows: parsed.Source()}
	if isValues {
		view.Values = parsed.ComputeWith(computeOptions(source, isUnconfined, limits, fetcher))
	}
	return writeJSON(streams.Out, view)
}

// runFromJSON reads a sheetView JSON (from parse) and writes its source rows as
// TSV — the inverse of parse, so a spreadsheet round-trips through JSON. Any
// computed values in the input are ignored; the source rows are authoritative.
func runFromJSON(streams Streams, source sourcePath, limits tsvsheet.Limits) error {
	reader, release, err := source.open(streams.In)
	if err != nil {
		return err
	}
	defer func() { _ = release() }()

	var view sheetView
	if err := json.NewDecoder(reader).Decode(&view); err != nil {
		return tsvsheet.ErrSyntax.With(err)
	}
	if err := vetGrid(view.Rows, limits); err != nil {
		return err
	}
	return tsvsheet.WriteTSV(streams.Out, view.Rows)
}

// vetGrid refuses a decoded grid whose cell count exceeds the resident
// ceiling — the JSON round trip obeys the same budget as every load
// (spec 018); the decode transient is bounded by the input's own bytes.
func vetGrid(rows tsvsheet.Grid, limits tsvsheet.Limits) error {
	var cells int64
	for _, row := range rows {
		cells += int64(len(row))
	}
	budget := limits.EffectiveResidentCells()
	if cells > budget {
		return tsvsheet.ErrDocTooLarge.With(nil,
			"cells", cells, "budget", budget,
			"hint", "raise --max-cells to accept the cost")
	}
	return nil
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
  tsv parse sheet.tsvt | jq '.rows[1]'
  tsv parse --value sheet.tsvt | jq '.values'
  tsv parse sheet.tsvt | tsv from-json   # round-trip
  cat sheet.tsvt | tsv parse`,
		Flags: append([]cli.Flag{
			&cli.BoolFlag{
				Name:        "value",
				Sources:     cli.EnvVars("TSV_PARSE_VALUE"),
				Usage:       "Include the computed grid as \"values\"",
				Destination: &isValues,
			},
			&cli.BoolFlag{
				Name:        flagAllowAnyPaths,
				Sources:     cli.EnvVars("TSV_ALLOW_ANY_PATHS"),
				Usage:       usageAllowAnyPaths,
				Destination: &isUnconfined,
			},
		}, append(importFlags(), dataFlags()...)...),
		Action: importedAction(
			func(s Streams, args positional, limits tsvsheet.Limits, fetcher tsvsheet.Fetcher) error {
				return runParse(s, args.at(0), valueOutput(isValues), pathAccess(isUnconfined), limits, fetcher)
			},
		),
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
  tsv parse sheet.tsvt | tsv from-json
  jq '.rows[0] |= ascii_upcase' data.json | tsv from-json`,
		Action: func(_ context.Context, c *cli.Command) error {
			streams := Streams{In: stdin, Out: c.Root().Writer, Err: stderr}
			return runFromJSON(streams, positional(c.Args().Slice()).at(0), maxCellsLimits(c))
		},
	}
}
