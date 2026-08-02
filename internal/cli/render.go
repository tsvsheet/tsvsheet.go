package cli

import (
	"path/filepath"
	"time"

	"github.com/tsvsheet/go-tsvsheet"
	"github.com/urfave/cli/v3"

	"github.com/tsvsheet/tsvsheet.go/internal/loader"
)

// runRender parses the spreadsheet, computes it (resolving SHEET(...) references
// when the source is a file), and writes the resulting value grid in the chosen
// format (TSV by default). Errors go to the caller (and thence stderr); stdout
// carries only the computed grid, so render composes in unix pipelines.
func runRender(
	streams Streams,
	source sourcePath,
	outputFormat Format,
	hidden hiddenPolicy,
	isUnconfined pathAccess,
	limits tsvsheet.Limits,
	fetcher tsvsheet.Fetcher,
) error {
	reader, release, err := source.open(streams.In)
	if err != nil {
		return err
	}
	defer func() { _ = release() }()

	doc, err := parseDocument(reader)
	if err != nil {
		return err
	}
	grid := doc.Sheet().ComputeWith(computeOptions(source, isUnconfined, limits, fetcher))
	view, _ := doc.View()
	seen, err := project(grid, view, hidden)
	if err != nil {
		return err
	}
	return format(streams.Out, seen, outputFormat)
}

// computeOptions builds the compute options for a source: a filesystem sheet
// loader rooted at the file's directory (so SHEET/"file"! resolve sibling
// sheets), or no loader for stdin (references resolve to #REF!). isUnconfined
// selects the confined or unconfined loader; limits bounds every allocation.
func computeOptions(
	source sourcePath,
	isUnconfined pathAccess,
	limits tsvsheet.Limits,
	fetcher tsvsheet.Fetcher,
) tsvsheet.ComputeOptions {
	if source.isStdin() {
		return tsvsheet.ComputeOptions{At: time.Now(), Limits: limits, Fetcher: fetcher}
	}
	path := filepath.Clean(string(source))
	return tsvsheet.ComputeOptions{
		At:      time.Now(),
		Loader:  sheetLoader(loader.Dir(filepath.Dir(path)), isUnconfined),
		Base:    tsvsheet.Path(filepath.Base(path)),
		Limits:  limits,
		Fetcher: fetcher,
	}
}

// renderCommand builds the `render` command.
func renderCommand() *cli.Command {
	isUnconfined := false
	outputFormat := string(formatTSV)
	hidden := string(hiddenKeep)
	return &cli.Command{
		Name:      cmdRender,
		Usage:     "Compute a spreadsheet and write the values (TSV, CSV, HTML, or Markdown).",
		ArgsUsage: argSheetOptional,
		Description: `Compute a .tsvt spreadsheet — a grid of literal and =formula cells — and
write the computed value grid to stdout. The sheet is positional; omitted or
"-" reads stdin. --format selects the serialization: tsv (the default), csv,
html (a <table>), or markdown (a pipe table; md is an alias).

A sheet's own hide directives are advisory to a viewport, not to a pipeline,
so hidden rows and columns are written out like any other data.
--hidden=drop asks for the projected artifact instead, with them removed.
Declared header rows are marked up where a format can carry them (HTML thead).

Examples:
  tsv render sheet.tsvt
  tsv render --format csv sheet.tsvt
  tsv render -f markdown sheet.tsvt
  cat sheet.tsvt | tsv render`,
		Flags: append([]cli.Flag{
			&cli.StringFlag{
				Name:        flagFormat,
				Aliases:     []string{"f"},
				Sources:     cli.EnvVars("TSV_FORMAT"),
				Value:       string(formatTSV),
				Usage:       usageFormat,
				Destination: &outputFormat,
			},
			&cli.StringFlag{
				Name:        flagHidden,
				Sources:     cli.EnvVars("TSV_HIDDEN"),
				Value:       string(hiddenKeep),
				Usage:       usageHidden,
				Destination: &hidden,
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
				return runRender(
					s, args.at(0), Format(outputFormat), hiddenPolicy(hidden),
					pathAccess(isUnconfined), limits, fetcher,
				)
			},
		),
	}
}
