package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/tsvsheet/go-tsvsheet"
	"github.com/urfave/cli/v3"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

// runCheck parses and statically checks a spreadsheet, writing one diagnostic
// per line to the error stream. It returns ErrSyntax on a parse failure
// (exit 2), ErrDiagnostics when the sheet parses but has findings (exit 1), or
// nil when clean (exit 0).
func runCheck(streams Streams, source sourcePath) error {
	reader, release, err := source.open(streams.In)
	if err != nil {
		return err
	}
	defer func() { _ = release() }()

	parsed, err := parseSheet(reader)
	if err != nil {
		return err
	}
	return reportDiagnostics(streams.Err, tsvsheet.Check(parsed))
}

// reportDiagnostics writes each diagnostic to w and returns ErrDiagnostics when
// any are present.
func reportDiagnostics(w io.Writer, diags []tsvsheet.Diagnostic) error {
	for _, d := range diags {
		_, _ = fmt.Fprintf(w, "%s: %s\n", d.Cell, d.Message)
	}
	if len(diags) > 0 {
		return constants.ErrDiagnostics.With(nil, "count", len(diags))
	}
	return nil
}

// isSyntaxError reports whether err is a formula syntax error (exit-code 2).
func isSyntaxError(err error) bool { return errors.Is(err, tsvsheet.ErrSyntax) }

// isDiagnostics reports whether err signals that check found diagnostics.
func isDiagnostics(err error) bool { return errors.Is(err, constants.ErrDiagnostics) }

// checkCommand builds the `check` command.
func checkCommand() *cli.Command {
	return &cli.Command{
		Name:      cmdCheck,
		Usage:     "Validate a spreadsheet and report diagnostics.",
		ArgsUsage: argSheetOptional,
		Description: `Parse and statically check a .tsvt spreadsheet (positional; omitted or "-"
reads stdin). Diagnostics (unknown functions, non-A1 references) are written
one per line to stderr. Exit status: 0 clean, 1 diagnostics found, 2 syntax
error.

Examples:
  tsv check sheet.tsvt
  cat sheet.tsvt | tsv check`,
		Action: streamAction(func(s Streams, args positional) error {
			return runCheck(s, args.at(0))
		}),
	}
}
