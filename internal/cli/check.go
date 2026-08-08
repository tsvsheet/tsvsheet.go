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

// checkSources are the sheets one check run covers — one per positional
// argument, or a lone stdin source when none were given.
type checkSources []sourcePath

// diagnosticLabel prefixes a diagnostic with the sheet that produced it, and is
// empty for a run covering a single sheet.
type diagnosticLabel string

// label prefixes a sheet's diagnostics with the sheet that produced them, and
// only when the run covers more than one: a single-sheet run stays terse, the
// same rule grep follows. Without it, diagnostics from several sheets arrive
// indistinguishable from each other.
func (s checkSources) label(source sourcePath) diagnosticLabel {
	if len(s) < 2 {
		return ""
	}
	return diagnosticLabel(source.display() + ":")
}

// runCheck parses and statically checks every sheet, writing one diagnostic per
// line to the error stream. Checking continues past a failing sheet — a gate
// that stopped at the first would hide the rest — and the run exits with the
// severest outcome any sheet produced: ErrSyntax on a parse failure (exit 2),
// ErrDiagnostics for findings (exit 1), or nil when every sheet is clean.
func runCheck(streams Streams, sources checkSources, limits tsvsheet.Limits) error {
	var worst error
	for _, source := range sources {
		worst = worseOutcome(worst, checkSheet(streams, source, sources.label(source), limits))
	}
	return worst
}

// checkSheet checks one sheet, labelling its diagnostics for the caller.
func checkSheet(streams Streams, source sourcePath, label diagnosticLabel, limits tsvsheet.Limits) error {
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
	return reportDiagnostics(streams.Err, label, append(viewDiags, tsvsheet.Check(doc.Sheet())...))
}

// worseOutcome keeps the severer of two check outcomes so one bad sheet decides
// the exit code no matter where it sat in the argument list.
func worseOutcome(a, b error) error {
	if severityOf(a) >= severityOf(b) {
		return a
	}
	return b
}

// severityOf ranks a sheet's outcome. The order mirrors exitCode: a syntax
// error is the loudest (exit 2), an unreadable sheet and findings both exit 1,
// and an unreadable sheet outranks findings because that sheet went unchecked
// altogether — TestRunCheckUnreadableSheetOutranksDiagnostics states it.
func severityOf(err error) int {
	switch {
	case err == nil:
		return 0
	case isDiagnostics(err):
		return 1
	case isSyntaxError(err):
		return 3
	default:
		return 2
	}
}

// reportDiagnostics writes each diagnostic to w and returns ErrDiagnostics when
// any are present. A finding about a cell is addressed by its cell; a finding
// about a view directive by its line, since a directive occupies a physical
// line and no grid row.
func reportDiagnostics(w io.Writer, label diagnosticLabel, diags []tsvsheet.Diagnostic) error {
	for _, d := range diags {
		_, _ = fmt.Fprintf(w, "%s%s: %s\n", label, locationOf(d), d.Message)
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
		Usage:     "Validate spreadsheets and report diagnostics.",
		ArgsUsage: argSheetsOptional,
		Description: `Parse and statically check .tsvt spreadsheets (positional; omitted or "-"
reads stdin). Diagnostics (unknown functions, non-A1 references) are written
one per line to stderr, prefixed with the sheet when more than one is given.

Every sheet is checked even if an earlier one fails, and the run exits with the
severest result: 0 clean, 1 diagnostics found or a sheet unreadable, 2 syntax
error.

Examples:
  tsv check sheet.tsvt
  tsv check *.tsvt
  cat sheet.tsvt | tsv check`,
		Action: func(_ context.Context, c *cli.Command) error {
			streams := Streams{In: stdin, Out: c.Root().Writer, Err: stderr}
			return runCheck(streams, positional(c.Args().Slice()).sheets(), maxCellsLimits(c))
		},
	}
}
