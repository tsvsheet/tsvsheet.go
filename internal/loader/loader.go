// Package loader is the filesystem tsvsheet.Loader for embedded sub-sheets and
// cross-sheet references. Two modes trade safety for reach:
//
//   - FS (the secure default) confines every reference to a root directory via
//     os.Root — a bare or relative path resolves within root, and an absolute
//     path, a `..` escape, or a symlink out of root is rejected (#REF!).
//   - Unconfined resolves any path — bare/relative against the referencing
//     sheet's own directory, absolute as given — for operators who deliberately
//     reference sheets outside the tree (enabled by a CLI flag).
//
// The engine (the go-tsvsheet library) stays filesystem-free; a frontend injects one of
// these loaders.
package loader

import (
	"io"
	"os"
	"path/filepath"

	"github.com/uplang/go-tsvsheet"
)

// Dir is the directory a top sheet's bare or relative references resolve
// against (and, for FS, the sandbox boundary).
type Dir string

// FS returns a tsvsheet.Loader confined to root via os.Root: a reference resolves
// relative to the referencing sheet's own directory, and anything escaping root
// (an absolute path, a `..` traversal, a symlink out) is refused. The root is
// opened per call, so an unopenable root surfaces as a load error (#REF!).
func FS(root Dir) tsvsheet.Loader {
	return func(base, ref tsvsheet.Path) (tsvsheet.Sheet, tsvsheet.Path, error) {
		confined, err := os.OpenRoot(string(root))
		if err != nil {
			return tsvsheet.Sheet{}, "", err
		}
		defer func() { _ = confined.Close() }()
		target := tsvsheet.Path(filepath.Clean(filepath.Join(filepath.Dir(string(base)), string(ref))))
		file, err := confined.Open(string(target))
		if err != nil {
			return tsvsheet.Sheet{}, "", err
		}
		defer func() { _ = file.Close() }()
		return parse(file, target)
	}
}

// Unconfined returns a tsvsheet.Loader that reads any path: an absolute reference
// as given, a bare or relative reference against the referencing sheet's own
// directory (the top sheet's against root). It is the opt-in escape hatch for
// referencing sheets outside the tree.
func Unconfined(root Dir) tsvsheet.Loader {
	return func(base, ref tsvsheet.Path) (tsvsheet.Sheet, tsvsheet.Path, error) {
		target := resolvePath(root, base, ref)
		file, err := os.Open(string(target))
		if err != nil {
			return tsvsheet.Sheet{}, "", err
		}
		defer func() { _ = file.Close() }()
		return parse(file, target)
	}
}

// parse reads and parses a sheet from file; target is its resolved path, used
// for cycle detection and as the base for the sub-sheet's own references.
func parse(file io.Reader, target tsvsheet.Path) (tsvsheet.Sheet, tsvsheet.Path, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return tsvsheet.Sheet{}, "", err
	}
	parsed, err := tsvsheet.Parse(data)
	return parsed, target, err
}

// resolvePath resolves ref to a cleaned filesystem path (Unconfined): an
// absolute ref as given; a bare or relative ref against the referencing sheet's
// directory (or root, for the top sheet whose base is a bare filename).
func resolvePath(root Dir, base, ref tsvsheet.Path) tsvsheet.Path {
	if filepath.IsAbs(string(ref)) {
		return tsvsheet.Path(filepath.Clean(string(ref)))
	}
	dir := filepath.Dir(string(base))
	if dir == "." {
		dir = string(root)
	}
	return tsvsheet.Path(filepath.Clean(filepath.Join(dir, string(ref))))
}
