package cli

import (
	"github.com/uplang/go-tsvsheet"

	"github.com/uplang/tsvsheet.go/internal/loader"
)

// pathAccess selects sheet-reference confinement: references stay within the
// sheet's own directory (the secure default), or reach any path when the
// operator opts in.
type pathAccess bool

// The --allow-any-paths flag: its name and (shared) usage text. The flag itself
// is declared inline in each command bound to that command's local bool.
const (
	flagAllowAnyPaths  = "allow-any-paths"
	usageAllowAnyPaths = `Allow sheet references (SHEET(…), "file"!A1) to reach any path — absolute or outside the sheet's directory; the default confines them to it`
)

// sheetLoader builds the loader for a sheet rooted at dir: confined to dir via
// os.Root by default, or reading any path when isUnconfined is set.
func sheetLoader(dir loader.Dir, isUnconfined pathAccess) tsvsheet.Loader {
	if isUnconfined {
		return loader.Unconfined(dir)
	}
	return loader.FS(dir)
}
