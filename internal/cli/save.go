package cli

import (
	"os"
	"path/filepath"

	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/session"
)

// saver builds the persist function: it writes the session's current source
// back to the spreadsheet file. The plain func() error is assignable to both
// serve.Saver and tui.Saver.
//
// The write is confined and atomic. Confined: it goes through os.Root on the
// sheet's own directory, so a sheet path that is a symlink out of that directory
// cannot be used to overwrite the file it points at. Atomic: the bytes land in a
// temporary file that is then renamed over the sheet, so an interrupted save
// leaves the previous sheet intact instead of a truncated one — a plain
// os.WriteFile truncates first and would destroy the sheet if the process died
// mid-write.
func saver(sess *session.Session, source sourcePath) func() error {
	rawDir, rawFile := filepath.Split(filepath.Clean(string(source)))
	dir := sheetDir(rawDir)
	if dir == "" {
		dir = "."
	}
	return func() error { return saveAtomic(dir, sheetName(rawFile), sess.Source()) }
}

// sheetDir is the directory a sheet lives in — the confinement root a save
// writes through, so nothing outside it can be reached.
type sheetDir string

// sheetName is a sheet's base name within its sheetDir.
type sheetName string

// saveAtomic writes data as file inside dir, via a temporary file in that same
// directory and a rename. Same-directory is required: a rename is only atomic
// within one filesystem.
func saveAtomic(dir sheetDir, file sheetName, data []byte) error {
	root, err := os.OpenRoot(string(dir))
	if err != nil {
		return tsvsheet.ErrWriteFile.With(err, string(dir))
	}
	defer func() { _ = root.Close() }()
	tmp := string(file) + tempSuffix
	if err := writeFileIn(root, tmp, data, filePerm); err != nil {
		return tsvsheet.ErrWriteFile.With(err, tmp)
	}
	if err := renameIn(root, tmp, string(file)); err != nil {
		_ = root.Remove(tmp)
		return tsvsheet.ErrWriteFile.With(err, string(file))
	}
	return nil
}

// tempSuffix names the staging file a save renames from.
const tempSuffix = ".tsv-save"

// The two filesystem operations a save performs, as package vars so a test can
// force each failure — Go offers no way to make a real write or rename fail at a
// direct call site. Each branch carries a contract the tests assert:
// TestSaveAtomic_WriteFailureLeavesTheSheetIntact and
// TestSaveAtomic_RenameFailureLeavesTheSheetIntactAndRemovesTheStagingFile check
// that whichever step fails, the previous sheet is still on disk and no staging
// file survives. These stubs are process-global, so tests that swap them restore
// them and run serially.
var (
	writeFileIn = func(root *os.Root, name string, data []byte, perm os.FileMode) error {
		return root.WriteFile(name, data, perm)
	}
	renameIn = func(root *os.Root, from, to string) error { return root.Rename(from, to) }
)

// filePerm is the mode for a saved spreadsheet file.
const filePerm = 0o600
