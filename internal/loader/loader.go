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

	"github.com/tsvsheet/go-tsvsheet"
)

// Dir is the directory a top sheet's bare or relative references resolve
// against (and, for FS, the sandbox boundary).
type Dir string

// sheetFile is the shape parse needs from an opened sheet: sequential reads
// for buffering, positional reads for the census, and a size. *os.File has
// it; the seam keeps every failure branch reachable from a test.
type sheetFile interface {
	io.Reader
	io.ReaderAt
	Stat() (os.FileInfo, error)
}

// FS returns a tsvsheet.Loader confined to root via os.Root: a reference resolves
// relative to the referencing sheet's own directory, and anything escaping root
// (an absolute path, a `..` traversal, a symlink out) is refused. The root is
// opened per call, so an unopenable root surfaces as a load error (#REF!).
// Every load is bounded by limits (spec 018): a referenced sheet over the
// resident budget refuses via the census — the side door pays the same toll
// as the front door, in O(index) memory before any buffering.
func FS(root Dir, limits tsvsheet.Limits) tsvsheet.Loader {
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
		return parse(file, target, limits)
	}
}

// Unconfined returns a tsvsheet.Loader that reads any path: an absolute reference
// as given, a bare or relative reference against the referencing sheet's own
// directory (the top sheet's against root). It is the opt-in escape hatch for
// referencing sheets outside the tree. Loads are budget-bounded exactly as
// FS's are.
func Unconfined(root Dir, limits tsvsheet.Limits) tsvsheet.Loader {
	return func(base, ref tsvsheet.Path) (tsvsheet.Sheet, tsvsheet.Path, error) {
		target := resolvePath(root, base, ref)
		file, err := os.Open(string(target))
		if err != nil {
			return tsvsheet.Sheet{}, "", err
		}
		defer func() { _ = file.Close() }()
		return parse(file, target, limits)
	}
}

// parse reads and parses a sheet from file bounded by limits; target is its
// resolved path, used for cycle detection and as the base for the sub-sheet's
// own references. The census vets the open file through ReadAt BEFORE any
// buffering — ReadAt is positional, so the read offset survives for the
// buffering that follows an in-budget verdict — and the buffered bytes parse
// through the bounded form, so a file that grew between the two is still
// refused.
func parse(file sheetFile, target tsvsheet.Path, limits tsvsheet.Limits) (tsvsheet.Sheet, tsvsheet.Path, error) {
	info, err := file.Stat()
	if err != nil {
		return tsvsheet.Sheet{}, "", err
	}
	census, err := tsvsheet.Census(tsvsheet.ByteSource{ReadAt: file, Size: info.Size()})
	if err != nil {
		return tsvsheet.Sheet{}, "", err
	}
	if census.Cells > limits.EffectiveResidentCells() {
		return tsvsheet.Sheet{}, "", tsvsheet.ErrDocTooLarge.With(nil,
			"cells", census.Cells, "budget", limits.EffectiveResidentCells(), "sheet", string(target))
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return tsvsheet.Sheet{}, "", err
	}
	parsed, err := tsvsheet.ParseWith(data, limits)
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
