package cli

import (
	"github.com/urfave/cli/v3"

	"github.com/uplang/tsvsheet.go/internal/sheet"
)

// renderConfig binds the render command's template and data source paths.
type renderConfig struct {
	template sourcePath
	data     sourcePath
}

// runRender computes the worksheet and writes the resulting grid as TSV to the
// output stream. Errors go to the caller (and thence stderr); stdout carries
// only the computed grid, so render composes in unix pipelines.
func runRender(streams Streams, cfg renderConfig) error {
	templateReader, dataReader, release, err := templateAndData(cfg.template, cfg.data, streams.In)
	if err != nil {
		return err
	}
	defer func() { _ = release() }()

	out, err := computeWorksheet(templateReader, dataReader)
	if err != nil {
		return err
	}
	return sheet.WriteTSV(streams.Out, out)
}

// renderCommand builds the `render` command.
func renderCommand() *cli.Command {
	cfg := renderConfig{}
	return &cli.Command{
		Name:      "render",
		Usage:     "Compute a worksheet and write the result as TSV.",
		ArgsUsage: " ",
		Description: `Compute a .tsvt template against a .tsv data grid and write the computed
sheet as TSV to stdout.

When exactly one of --template/--data is a file, the other is read from stdin;
"-" selects stdin explicitly. Both cannot come from stdin.

Examples:
  tsvsheet render --template sheet.tsvt --data sheet.tsv
  cat sheet.tsvt | tsvsheet render --data sheet.tsv
  tsvsheet render --template sheet.tsvt < sheet.tsv`,
		Flags:  sourceFlags(&cfg.template, &cfg.data),
		Action: streamAction(func(s Streams) error { return runRender(s, cfg) }),
	}
}
