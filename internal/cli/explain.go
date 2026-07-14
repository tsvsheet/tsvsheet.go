package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/uplang/tsvsheet.go/internal/sheet"
)

// explainConfig binds the explain command's sources, target cell, and output
// form.
type explainConfig struct {
	template sourcePath
	data     sourcePath
	cell     string
	isJSON   bool
}

// jsonOutput selects the JSON rendering of a trace.
type jsonOutput bool

// runExplain traces how the target cell was computed, writing a human-readable
// report or JSON to the output stream.
func runExplain(streams Streams, cfg explainConfig) error {
	at, err := sheet.ParseAddress(sheet.AddressText(cfg.cell))
	if err != nil {
		return err
	}
	templateReader, dataReader, release, err := templateAndData(cfg.template, cfg.data, streams.In)
	if err != nil {
		return err
	}
	defer func() { _ = release() }()

	trace, err := traceCell(templateReader, dataReader, at)
	if err != nil {
		return err
	}
	return writeTrace(streams.Out, trace, jsonOutput(cfg.isJSON))
}

// traceCell parses, reads, and explains the target cell.
func traceCell(templateReader, dataReader io.Reader, at sheet.Address) (sheet.Trace, error) {
	tmpl, err := parseTemplate(templateReader)
	if err != nil {
		return sheet.Trace{}, err
	}
	grid, err := sheet.ReadTSV(dataReader)
	if err != nil {
		return sheet.Trace{}, err
	}
	return sheet.Explain(tmpl, grid, at)
}

// writeTrace renders a trace as JSON or a human-readable report.
func writeTrace(w io.Writer, trace sheet.Trace, isJSON jsonOutput) error {
	if isJSON {
		return writeJSON(w, traceJSON(trace))
	}
	return writeTraceText(w, trace)
}

// writeTraceText writes the human-readable trace report.
func writeTraceText(w io.Writer, trace sheet.Trace) error {
	_, _ = fmt.Fprintf(w, "%s = %s\n", trace.Cell.String(), trace.Value)
	if trace.Formula != "" {
		_, _ = fmt.Fprintf(w, "  formula: %s\n", trace.Formula)
	}
	for _, in := range trace.Inputs {
		_, _ = fmt.Fprintf(w, "  %s = %s\n", in.Ref, in.Value)
	}
	return nil
}

// traceJSON is the JSON projection of a trace.
type traceView struct {
	Cell    string           `json:"cell"`
	Value   string           `json:"value"`
	Formula string           `json:"formula,omitempty"`
	Inputs  []traceInputView `json:"inputs,omitempty"`
}

// traceInputView is the JSON projection of one trace input.
type traceInputView struct {
	Ref   string `json:"ref"`
	Value string `json:"value"`
}

// traceJSON builds the JSON view of a trace.
func traceJSON(trace sheet.Trace) traceView {
	inputs := make([]traceInputView, len(trace.Inputs))
	for i, in := range trace.Inputs {
		inputs[i] = traceInputView{Ref: in.Ref, Value: in.Value}
	}
	view := traceView{Cell: trace.Cell.String(), Value: trace.Value, Formula: trace.Formula}
	if len(inputs) > 0 {
		view.Inputs = inputs
	}
	return view
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
		Usage:     "Trace how one computed cell was produced.",
		ArgsUsage: "<cell> [template] [data]",
		Description: `Explain a single output cell: its value, the formula that produced it, and
the resolved value of each reference the formula reads. The cell is required
and positional; the template and data follow (omitted or "-" reads stdin).

Examples:
  tsvsheet explain F4 sheet.tsvt sheet.tsv
  tsvsheet explain F4 --json sheet.tsvt sheet.tsv`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        jsonFlag,
				Usage:       "Emit the trace as JSON",
				Destination: &cfg.isJSON,
			},
		},
		Action: streamAction(func(s Streams, args positional) error {
			cfg.cell = args.text(0)
			cfg.template = args.at(1)
			cfg.data = args.at(2)
			return runExplain(s, cfg)
		}),
	}
}
