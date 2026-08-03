package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/tsvsheet/go-tsvsheet"
	"github.com/urfave/cli/v3"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

// runCheck parses and statically checks a spreadsheet, writing one diagnostic
// per line to the error stream. It returns ErrSyntax on a parse failure
// (exit 2), ErrDiagnostics when the sheet parses but has findings (exit 1), or
// nil when clean (exit 0).
func runCheck(streams Streams, source sourcePath, limits tsvsheet.Limits) error {
	reader, release, err := source.open(streams.In)
	if err != nil {
		return err
	}
	defer func() { _ = release() }()

	doc, err := parseDocument(reader, limits)
	if err != nil {
		return err
	}
	_, viewDiags := doc.View()
	return reportDiagnostics(streams.Err, append(viewDiags, tsvsheet.Check(doc.Sheet())...))
}

// reportDiagnostics writes each diagnostic to w and returns ErrDiagnostics when
// any are present. A finding about a cell is addressed by its cell; a finding
// about a view directive by its line, since a directive occupies a physical
// line and no grid row.
func reportDiagnostics(w io.Writer, diags []tsvsheet.Diagnostic) error {
	for _, d := range diags {
		_, _ = fmt.Fprintf(w, "%s: %s\n", locationOf(d), d.Message)
	}
	if len(diags) > 0 {
		return constants.ErrDiagnostics.With(nil, "count", len(diags))
	}
	return nil
}

// locationOf names where a diagnostic was found: a cell address, or a line for
// the directive findings that have no cell.
func locationOf(d tsvsheet.Diagnostic) string {
	if d.Cell != "" {
		return d.Cell
	}
	return "line " + strconv.Itoa(d.Line)
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
		Action: func(_ context.Context, c *cli.Command) error {
			streams := Streams{In: stdin, Out: c.Root().Writer, Err: stderr}
			return runCheck(streams, positional(c.Args().Slice()).at(0), maxCellsLimits(c))
		},
	}
}
