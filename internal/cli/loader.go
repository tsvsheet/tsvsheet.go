package cli

import (
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/loader"
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
// Loads are bounded by limits (spec 018) — a referenced sheet pays the same
// budget as the sheet that names it.
func sheetLoader(dir loader.Dir, isUnconfined pathAccess, limits tsvsheet.Limits) tsvsheet.Loader {
	if isUnconfined {
		return loader.Unconfined(dir, limits)
	}
	return loader.FS(dir, limits)
}
