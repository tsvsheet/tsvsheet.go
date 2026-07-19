package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/tsvsheet/go-tsvsheet"
	"github.com/urfave/cli/v3"
)

// explainConfig binds the explain command's source, target cell, and output
// form.
type explainConfig struct {
	source sourcePath
	cell   string
	isJSON bool
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
	trace, err := tsvsheet.Explain(parsed, at)
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
	return nil
}

// writeJSON encodes v as indented JSON followed by a newline.
func writeJSON(w io.Writer, v any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

// explainCommand builds the `explain` command.
func explainCommand() *cli.Command {
	cfg := explainConfig{}
	return &cli.Command{
		Name:      cmdExplain,
		Usage:     "Trace how one cell was computed.",
		ArgsUsage: "<cell> [sheet]",
		Description: `Explain a single cell: its value, the formula that produced it (empty for a
literal), and the resolved value of each cell the formula reads. The cell is
required and positional; the sheet follows (omitted or "-" reads stdin).

Examples:
  tsv explain D2 sheet.tsvt
  tsv explain D2 --json sheet.tsvt`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        jsonFlag,
				Usage:       "Emit the trace as JSON",
				Destination: &cfg.isJSON,
			},
		},
		Action: streamAction(func(s Streams, args positional) error {
			cfg.cell = args.text(0)
			cfg.source = args.at(1)
			return runExplain(s, cfg)
		}),
	}
}
