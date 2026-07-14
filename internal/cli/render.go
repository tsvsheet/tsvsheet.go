package cli

import (
	"github.com/urfave/cli/v3"

	"github.com/uplang/tsvsheet.go/internal/sheet"
)

// runRender computes the worksheet and writes the resulting grid as TSV to the
// output stream. Errors go to the caller (and thence stderr); stdout carries
// only the computed grid, so render composes in unix pipelines.
func runRender(streams Streams, template, data sourcePath) error {
	templateReader, dataReader, release, err := templateAndData(template, data, streams.In)
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
	return &cli.Command{
		Name:      cmdRender,
		Usage:     "Compute a worksheet and write the result as TSV.",
		ArgsUsage: "[template] [data]",
		Description: `Compute a .tsvt template against a .tsv data grid and write the computed
sheet as TSV to stdout.

The template and data are positional. An omitted argument is read from stdin
(so a redirect or pipe fills it); both cannot come from stdin at once. To pipe
the template while naming a data file, use the /dev/stdin path.

Examples:
  tsvsheet render sheet.tsvt sheet.tsv
  tsvsheet render sheet.tsvt < sheet.tsv
  cat sheet.tsvt | tsvsheet render /dev/stdin sheet.tsv`,
		Action: streamAction(func(s Streams, args positional) error {
			return runRender(s, args.at(0), args.at(1))
		}),
	}
}
