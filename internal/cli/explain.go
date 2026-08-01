package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/tsvsheet/go-tsvsheet"
	"github.com/urfave/cli/v3"
)

// explainConfig binds the explain command's source, target cell, output form,
// and the compute environment a trace resolves against.
type explainConfig struct {
	fetcher      tsvsheet.Fetcher
	source       sourcePath
	cell         string
	limits       tsvsheet.Limits
	isJSON       bool
	isUnconfined pathAccess
}

// jsonOutput selects the JSON rendering of a trace.
type jsonOutput bool

// runExplain traces how the target cell was computed, writing a human-readable
// report or JSON to the output stream.
func runExplain(streams Streams, cfg explainConfig) error {
	at, err := tsvsheet.ParseAddress(tsvsheet.AddressText(cfg.cell))
	if err != nil {
		return err
	}
	reader, release, err := cfg.source.open(streams.In)
	if err != nil {
		return err
	}
	defer func() { _ = release() }()

	parsed, err := parseSheet(reader)
	if err != nil {
		return err
	}
	trace, err := tsvsheet.ExplainWith(parsed, at,
		computeOptions(cfg.source, cfg.isUnconfined, cfg.limits, cfg.fetcher))
	if err != nil {
		return err
	}
	return writeTrace(streams.Out, trace, jsonOutput(cfg.isJSON))
}

// writeTrace renders a trace as JSON or a human-readable report.
func writeTrace(w io.Writer, trace tsvsheet.Trace, isJSON jsonOutput) error {
	if isJSON {
		return writeJSON(w, trace)
	}
	return writeTraceText(w, trace)
}

// writeTraceText writes the human-readable trace report.
func writeTraceText(w io.Writer, trace tsvsheet.Trace) error {
	_, _ = fmt.Fprintf(w, "%s = %s\n", trace.Cell, trace.Value)
	if trace.Formula != "" {
		_, _ = fmt.Fprintf(w, "  formula: %s\n", trace.Formula)
	}
	for _, in := range trace.Inputs {
		_, _ = fmt.Fprintf(w, "  %s = %s\n", in.Ref, in.Value)
	}
	writeTraceImports(w, trace.Imports)
	writeTraceNotes(w, trace.Notes)
	return nil
}

// writeTraceNotes reports where this cell reads differently than the same text
// would in a conventional spreadsheet. `check` names these at authoring time;
// naming them here too means a reader who has not run check still learns why
// the cell holds the value it holds, which is the whole point of announcing a
// divergence rather than leaving it to be discovered from a surprising total.
func writeTraceNotes(w io.Writer, notes []string) {
	for _, note := range notes {
		_, _ = fmt.Fprintf(w, "  note: %s\n", note)
	}
}

// writeTraceImports reports where each IMPORT* in the formula went. Every import
// failure is the same #IMPORT! in the grid, so this is the only place the
// specific reason is visible.
func writeTraceImports(w io.Writer, imports []tsvsheet.TraceImport) {
	for _, imp := range imports {
		_, _ = fmt.Fprintf(w, "  import %s -> %s\n", imp.Source, importOutcome(imp))
	}
}

// importOutcome renders one import's result: the URL it reached, or why it
// did not.
func importOutcome(imp tsvsheet.TraceImport) string {
	if imp.Error != "" {
		return "FAILED: " + imp.Error
	}
	return imp.URL
}

// writeJSON encodes v as indented JSON followed by a newline.
func writeJSON(w io.Writer, v any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

// explainCommand builds the `explain` command.
func explainCommand() *cli.Command {
	isUnconfined := false
	cfg := explainConfig{}
	return &cli.Command{
		Name:      cmdExplain,
		Usage:     "Trace how one cell was computed.",
		ArgsUsage: "<cell> [sheet]",
		Description: `Explain a single cell: its value, the formula that produced it (empty for a
literal), and the resolved value of each cell the formula reads. The cell is
required and positional; the sheet follows (omitted or "-" reads stdin).

An IMPORT* formula also reports where each import went — the URL it resolved
to, or the specific reason it failed. Every import failure is the same opaque
#IMPORT! in the grid, so this is the only place a denied host and a traversal
above the data base look different.

A cell whose formula reads differently here than the same text would in a
conventional spreadsheet carries a "note:" line saying so — =-2^2 is -4 here and
4 in Excel — so the difference is something you are told rather than something
you find later in a wrong total. tsv check reports the same at authoring time.

Examples:
  tsv explain D2 sheet.tsvt
  tsv explain D2 --json sheet.tsvt
  tsv explain A1 --data ./data sheet.tsvt`,
		Flags: append([]cli.Flag{
			&cli.BoolFlag{
				Name:        jsonFlag,
				Usage:       "Emit the trace as JSON",
				Destination: &cfg.isJSON,
			},
			&cli.BoolFlag{Name: flagAllowAnyPaths, Usage: usageAllowAnyPaths, Destination: &isUnconfined},
		}, append(importFlags(), dataFlags()...)...),
		Action: func(_ context.Context, c *cli.Command) error {
			base, closeData, err := resolveData(c)
			if err != nil {
				return err
			}
			defer func() { _ = closeData() }()
			fetcher, _, err := resolveImport(c, base)
			if err != nil {
				return err
			}
			args := positional(c.Args().Slice())
			cfg.cell = args.text(0)
			if cfg.cell == "" {
				return missingArgument(c)
			}
			cfg.source = args.at(1)
			cfg.isUnconfined = pathAccess(isUnconfined)
			cfg.limits = maxCellsLimits(c)
			cfg.fetcher = fetcher
			return runExplain(Streams{In: stdin, Out: c.Root().Writer, Err: stderr}, cfg)
		},
	}
}
