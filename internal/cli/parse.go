package cli

import (
	"github.com/urfave/cli/v3"

	"github.com/uplang/tsvsheet.go/internal/sheet"
	"github.com/uplang/tsvsheet.go/internal/tsvt"
)

// parseConfig binds the parse command's template source path.
type parseConfig struct {
	template sourcePath
}

// lineView is the JSON projection of one parsed template line: its kind and its
// normalized source form (cells for a row).
type lineView struct {
	Kind   sheet.LineKind `json:"kind"`
	Line   int            `json:"line"`
	Source string         `json:"source"`
	Cells  []string       `json:"cells,omitempty"`
}

// worksheetView is the JSON projection of a parsed template.
type worksheetView struct {
	Lines []lineView `json:"lines"`
}

// runParse parses a template and writes its structure as JSON to the output
// stream — a stable, jq-friendly surface for scripting.
func runParse(streams Streams, cfg parseConfig) error {
	reader, release, err := cfg.template.open(streams.In)
	if err != nil {
		return err
	}
	defer func() { _ = release() }()

	tmpl, err := parseTemplate(reader)
	if err != nil {
		return err
	}
	return writeJSON(streams.Out, worksheetView{Lines: lineViews(tmpl)})
}

// lineViews projects every template line to its JSON view.
func lineViews(tmpl tsvt.Template) []lineView {
	views := make([]lineView, len(tmpl.Lines))
	for i, line := range tmpl.Lines {
		views[i] = lineViewOf(line)
	}
	return views
}

// lineViewOf builds one line's JSON view, listing rendered cells for a row.
func lineViewOf(line tsvt.Line) lineView {
	view := lineView{Kind: sheet.LineKindOf(line), Line: int(lineNumberOf(line)), Source: sheet.RenderLine(line)}
	if row, ok := line.(tsvt.Row); ok {
		view.Cells = cellSources(row)
	}
	return view
}

// cellSources renders each cell of a row to its source form.
func cellSources(row tsvt.Row) []string {
	cells := make([]string, len(row.Cells))
	for i, cell := range row.Cells {
		cells[i] = sheet.RenderCell(cell)
	}
	return cells
}

// lineNumberOf returns a line's 1-based source position.
func lineNumberOf(line tsvt.Line) tsvt.LineNumber {
	switch l := line.(type) {
	case tsvt.HeaderMarker:
		return l.At
	case tsvt.BodyMarker:
		return l.At
	case tsvt.FinalMarker:
		return l.At
	case tsvt.Structural:
		return l.At
	default: // tsvt.Row
		return l.(tsvt.Row).At
	}
}

// parseCommand builds the `parse` command.
func parseCommand() *cli.Command {
	cfg := parseConfig{}
	return &cli.Command{
		Name:      "parse",
		Usage:     "Parse a template and emit its structure as JSON.",
		ArgsUsage: " ",
		Description: `Parse a .tsvt template and write its line structure as JSON to stdout — a
stable surface for scripting and tooling.

Examples:
  tsvsheet parse --template sheet.tsvt | jq '.lines[].kind'
  cat sheet.tsvt | tsvsheet parse`,
		Flags:  templateFlagOnly(&cfg.template),
		Action: streamAction(func(s Streams) error { return runParse(s, cfg) }),
	}
}
