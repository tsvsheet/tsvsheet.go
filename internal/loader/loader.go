// Package loader is the filesystem sheet.Loader for embedded sub-sheets and
// cross-sheet references: it resolves a path (bare, relative, or absolute) to a
// parsed sheet. A bare or relative path resolves against the embedding sheet's
// own directory (the top sheet's against the root the frontend fixes); an
// absolute path is read as given. The engine (internal/sheet) stays
// filesystem-free; this package is injected by the frontends (serve, cli).
package loader

import (
	"os"
	"path/filepath"

	"github.com/uplang/tsvsheet.go/internal/sheet"
)

// Dir is the directory a top sheet's bare or relative references resolve
// against.
type Dir string

// FS returns a sheet.Loader rooted at root: it resolves the reference to a
// filesystem path, reads it, and parses it. The resolved (cleaned) path is
// returned for cycle detection and as the base for the sub-sheet's own
// references.
func FS(root Dir) sheet.Loader {
	return func(base, ref sheet.Path) (sheet.Sheet, sheet.Path, error) {
		target := resolvePath(root, base, ref)
		data, err := os.ReadFile(string(target))
		if err != nil {
			return sheet.Sheet{}, "", err
		}
		parsed, err := sheet.Parse(data)
		return parsed, target, err
	}
}

// resolvePath resolves ref to a cleaned filesystem path: an absolute ref as
// given; a bare or relative ref against the embedding sheet's directory (or
// root, for the top sheet whose base is a bare filename).
func resolvePath(root Dir, base, ref sheet.Path) sheet.Path {
	if filepath.IsAbs(string(ref)) {
		return sheet.Path(filepath.Clean(string(ref)))
	}
	dir := filepath.Dir(string(base))
	if dir == "." {
		dir = string(root)
	}
	return sheet.Path(filepath.Clean(filepath.Join(dir, string(ref))))
}
