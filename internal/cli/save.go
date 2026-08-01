package cli

import (
	"os"
	"strconv"
	"sync/atomic"

	"github.com/tsvsheet/go-tsvsheet"
)

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
	tmp := stagingName(file)
	if err := writeFileIn(root, tmp, data, modeOf(root, file)); err != nil {
		return tsvsheet.ErrWriteFile.With(err, tmp)
	}
	if err := renameIn(root, tmp, string(file)); err != nil {
		_ = root.Remove(tmp)
		return tsvsheet.ErrWriteFile.With(err, string(file))
	}
	return nil
}

// stagingName is the per-save staging file a rename promotes. It carries the
// process id and a counter because a fixed name is shared state: two saves of
// one sheet running at once would write the same staging file and rename it
// twice, so one process could publish the other's bytes — and the loser's
// rename fails after the winner already consumed the file, reporting a failure
// for a sheet that did change.
func stagingName(file sheetName) string {
	return string(file) + tempSuffix + "." + strconv.Itoa(os.Getpid()) + "." + strconv.FormatUint(nextStagingID(), 36)
}

// stagingCounter distinguishes concurrent saves within one process.
var stagingCounter atomic.Uint64

// nextStagingID hands out a process-unique staging discriminator.
func nextStagingID() uint64 { return stagingCounter.Add(1) }

// modeOf is the mode a save writes: the sheet's current mode when it exists,
// so an ordinary 0644 sheet is not silently tightened to 0600 by an edit, and
// the private default for a sheet being created.
func modeOf(root *os.Root, file sheetName) os.FileMode {
	info, err := root.Stat(string(file))
	if err != nil {
		return filePerm
	}
	return info.Mode().Perm()
}

// tempSuffix prefixes the staging file a save renames from.
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
