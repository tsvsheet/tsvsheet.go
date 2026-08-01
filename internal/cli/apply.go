package cli

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/tsvsheet/go-tsvsheet"
	"github.com/urfave/cli/v3"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

// applyDryRun selects whether an apply only dry-runs the fold (--check)
// instead of writing the result.
type applyDryRun bool

// The two apply modes, named at the call sites.
const (
	applyWrite     applyDryRun = false
	applyCheckOnly applyDryRun = true
)

// noBaseNotice is the stderr warning for an unconditional batch — an edits
// document naming no `#.base` revision applies to whatever the sheet now is.
const noBaseNotice = "edits name no base revision; applying unconditionally"

// runApply reads the sheet and the edits document, applies the deterministic
// fold, and persists the result: a sheet file is replaced atomically and the
// new revision printed to out; a stdin sheet writes the resulting document to
// out and the revision to the error stream. In applyCheckOnly mode nothing is
// written; the would-be revision prints to out. Every refusal — malformed
// edits, base mismatch, refused op — is the engine's own sentinel, and the
// sheet is never touched on any failure.
func runApply(streams Streams, sheet, edits sourcePath, limits tsvsheet.Limits, isDryRun applyDryRun) error {
	if sheet.isStdin() && edits.isStdin() {
		return constants.ErrApplyBothStdin.With(nil)
	}
	doc, batch, err := readApplyInputs(streams, sheet, edits)
	if err != nil {
		return err
	}
	if batch.Base() == "" {
		_, _ = fmt.Fprintln(streams.Err, noBaseNotice)
	}
	out, err := tsvsheet.Apply(doc, batch, limits)
	if err != nil {
		return err
	}
	return writeApplyResult(streams, sheet, out, isDryRun)
}

// readApplyInputs opens and parses the sheet document and the edits batch.
func readApplyInputs(streams Streams, sheet, edits sourcePath) (tsvsheet.Document, tsvsheet.Edits, error) {
	doc, err := readApplySheet(streams, sheet)
	if err != nil {
		return tsvsheet.Document{}, tsvsheet.Edits{}, err
	}
	reader, release, err := edits.open(streams.In)
	if err != nil {
		return tsvsheet.Document{}, tsvsheet.Edits{}, err
	}
	defer func() { _ = release() }()
	data, err := readAll(reader)
	if err != nil {
		return tsvsheet.Document{}, tsvsheet.Edits{}, err
	}
	batch, err := tsvsheet.ParseEdits(data)
	if err != nil {
		return tsvsheet.Document{}, tsvsheet.Edits{}, err
	}
	return doc, batch, nil
}

// readApplySheet opens and parses the sheet positional.
func readApplySheet(streams Streams, sheet sourcePath) (tsvsheet.Document, error) {
	reader, release, err := sheet.open(streams.In)
	if err != nil {
		return tsvsheet.Document{}, err
	}
	defer func() { _ = release() }()
	return parseDocument(reader)
}

// writeApplyResult persists the applied document per the sheet source and
// mode, printing the resulting revision.
func writeApplyResult(streams Streams, sheet sourcePath, out tsvsheet.Document, isDryRun applyDryRun) error {
	revision := string(tsvsheet.Revision(out))
	if isDryRun == applyCheckOnly {
		_, _ = fmt.Fprintln(streams.Out, revision)
		return nil
	}
	if sheet.isStdin() {
		_, _ = streams.Out.Write(out.Text())
		_, _ = fmt.Fprintln(streams.Err, revision)
		return nil
	}
	if err := saveSheetFile(sheet, out.Text()); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(streams.Out, revision)
	return nil
}

// saveSheetFile replaces the sheet file atomically (temp + rename), confined
// to the sheet's own directory — the saver contract, for a one-shot write.
func saveSheetFile(sheet sourcePath, data []byte) error {
	rawDir, rawFile := filepath.Split(filepath.Clean(string(sheet)))
	dir := sheetDir(rawDir)
	if dir == "" {
		dir = "."
	}
	return saveAtomic(dir, sheetName(rawFile), data)
}

// flagApplyCheck names the dry-run flag.
const flagApplyCheck = "check"

// applyCommand builds the `apply` command.
func applyCommand() *cli.Command {
	return &cli.Command{
		Name:      cmdApply,
		Usage:     "Apply an edits document to a spreadsheet.",
		ArgsUsage: "<sheet> [edits]",
		Description: `Apply an edits document — a TSV stream of semantic operations (setCell,
insertRow, deleteRow, insertCol, deleteCol, duplicateRow, duplicateCol, fill,
paste) — to a .tsvt spreadsheet, atomically: any refused line rejects the whole
batch and the sheet is untouched. The edits positional (omitted or "-") reads
stdin; a "#.base <revision>" metadata line makes the batch conditional — it is
refused when the sheet's content revision differs. A sheet of "-" reads stdin
and writes the resulting document to stdout (revision to stderr) instead of
writing a file. On success the sheet's new content revision (the lowercase-hex
SHA-256 of its canonical bytes) is printed.

Examples:
  tsv apply sheet.tsvt batch.edits
  printf 'setCell\tB7\t=sum(B1:B6)\n' | tsv apply sheet.tsvt
  tsv apply --check sheet.tsvt batch.edits`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  flagApplyCheck,
				Usage: "dry-run: verify the base and fold, print the would-be revision, write nothing",
			},
		},
		Action: func(_ context.Context, c *cli.Command) error {
			streams := Streams{In: stdin, Out: c.Root().Writer, Err: stderr}
			args := positional(c.Args().Slice())
			return runApply(streams, args.at(0), args.at(1), maxCellsLimits(c), applyDryRun(c.Bool(flagApplyCheck)))
		},
	}
}
