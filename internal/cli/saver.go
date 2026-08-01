// The persist function a long-running frontend holds: serve and tui both save
// through it, so a session's writes land the same way a one-shot apply's do.
package cli

import (
	"path/filepath"

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
