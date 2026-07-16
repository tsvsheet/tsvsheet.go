package cli

import (
	"path/filepath"
	"time"

	"github.com/uplang/go-tsvsheet"
	"github.com/urfave/cli/v3"

	"github.com/uplang/tsvsheet.go/internal/loader"
)

// runRender parses the spreadsheet, computes it (resolving SHEET(...) references
// when the source is a file), and writes the resulting value grid as TSV.
// Errors go to the caller (and thence stderr); stdout carries only the computed
// grid, so render composes in unix pipelines.
func runRender(
	streams Streams,
	source sourcePath,
	isUnconfined pathAccess,
	limits tsvsheet.Limits,
	fetcher tsvsheet.Fetcher,
) error {
	reader, release, err := source.open(streams.In)
	if err != nil {
		return err
	}
	defer func() { _ = release() }()

	parsed, err := parseSheet(reader)
	if err != nil {
		return err
	}
	return tsvsheet.WriteTSV(streams.Out, parsed.ComputeWith(computeOptions(source, isUnconfined, limits, fetcher)))
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
	return &cli.Command{
		Name:      cmdRender,
		Usage:     "Compute a spreadsheet and write the values as TSV.",
		ArgsUsage: argSheetOptional,
		Description: `Compute a .tsvt spreadsheet — a grid of literal and =formula cells — and
write the computed value grid as TSV to stdout. The sheet is positional;
omitted or "-" reads stdin.

Examples:
  tsvsheet render sheet.tsvt
  cat sheet.tsvt | tsvsheet render`,
		Flags: append([]cli.Flag{
			&cli.BoolFlag{Name: flagAllowAnyPaths, Usage: usageAllowAnyPaths, Destination: &isUnconfined},
		}, importFlags()...),
		Action: importedAction(
			func(s Streams, args positional, limits tsvsheet.Limits, fetcher tsvsheet.Fetcher) error {
				return runRender(s, args.at(0), pathAccess(isUnconfined), limits, fetcher)
			},
		),
	}
}
