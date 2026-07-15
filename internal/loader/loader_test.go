package loader_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/loader"
	"github.com/uplang/tsvsheet.go/internal/sheet"
)

// write creates a file (making parent directories) under dir.
func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestFS_BareResolvesAgainstRoot(t *testing.T) {
	t.Parallel()

	// A bare reference from the top sheet (base has no directory) resolves in
	// the root, and the resolved path is returned for the sub-sheet's own refs.
	dir := t.TempDir()
	write(t, dir, "child.tsvt", "=output(42)\n")
	ld := loader.FS(loader.Dir(dir))

	sub, resolved, err := ld("main.tsvt", "child.tsvt")
	require.NoError(t, err)
	assert.Equal(t, sheet.Path(filepath.Join(dir, "child.tsvt")), resolved)
	assert.Equal(t, "42", sub.Compute()[0][0])
}

func TestFS_RelativeFromSubdirBase(t *testing.T) {
	t.Parallel()

	// A relative reference from a sub-sheet whose base carries a directory
	// resolves against that directory (the dir != "." branch).
	dir := t.TempDir()
	write(t, dir, "sub/leaf.tsvt", "=output(7)\n")
	ld := loader.FS(loader.Dir(dir))

	_, resolved, err := ld(sheet.Path(filepath.Join(dir, "sub", "mid.tsvt")), "leaf.tsvt")
	require.NoError(t, err)
	assert.Equal(t, sheet.Path(filepath.Join(dir, "sub", "leaf.tsvt")), resolved)
}

func TestFS_AbsoluteReference(t *testing.T) {
	t.Parallel()

	// An absolute reference is read as given, regardless of base or root.
	dir := t.TempDir()
	abs := write(t, dir, "abs.tsvt", "=output(1)\n")
	ld := loader.FS(loader.Dir(t.TempDir())) // an unrelated root

	_, resolved, err := ld("main.tsvt", sheet.Path(abs))
	require.NoError(t, err)
	assert.Equal(t, sheet.Path(abs), resolved)
}

func TestFS_MissingFile(t *testing.T) {
	t.Parallel()

	ld := loader.FS(loader.Dir(t.TempDir()))
	_, _, err := ld("main.tsvt", "absent.tsvt")
	require.Error(t, err)
}

func TestFS_ParseError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write(t, dir, "bad.tsvt", "=sum(\n") // malformed formula
	ld := loader.FS(loader.Dir(dir))

	_, _, err := ld("main.tsvt", "bad.tsvt")
	require.Error(t, err)
}
