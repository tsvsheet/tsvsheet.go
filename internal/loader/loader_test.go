package loader_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/loader"
)

// write creates a file (making parent directories) under dir.
func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// ---- FS (confined via os.Root) ------------------------------------------

func TestFS_ConfinedResolvesWithinRoot(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write(t, dir, "child.tsvt", "=output(42)\n")
	ld := loader.FS(loader.Dir(dir), tsvsheet.DefaultLimits())

	sub, resolved, err := ld("main.tsvt", "child.tsvt")
	require.NoError(t, err)
	assert.Equal(t, tsvsheet.Path("child.tsvt"), resolved) // root-relative
	assert.Equal(t, "42", sub.Compute()[0][0])
}

func TestFS_AbsoluteAndEscapeAreRefused(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write(t, dir, "in.tsvt", "1\n")
	ld := loader.FS(loader.Dir(dir), tsvsheet.DefaultLimits())

	_, _, absErr := ld("main.tsvt", "/etc/hosts")
	require.Error(t, absErr) // os.Root refuses an absolute path
	_, _, upErr := ld("main.tsvt", "../escape.tsvt")
	require.Error(t, upErr) // …and a `..` traversal out of root
}

func TestFS_MissingFile(t *testing.T) {
	t.Parallel()

	ld := loader.FS(loader.Dir(t.TempDir()), tsvsheet.DefaultLimits())
	_, _, err := ld("main.tsvt", "absent.tsvt")
	require.Error(t, err)
}

func TestFS_OpenRootFails(t *testing.T) {
	t.Parallel()

	ld := loader.FS(loader.Dir(filepath.Join(t.TempDir(), "does-not-exist")), tsvsheet.DefaultLimits())
	_, _, err := ld("main.tsvt", "child.tsvt")
	require.Error(t, err)
}

func TestFS_ReadAllErrorOnDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "nested"), 0o750))
	ld := loader.FS(loader.Dir(dir), tsvsheet.DefaultLimits())

	_, _, err := ld("main.tsvt", "nested") // opens, but reading a directory fails
	require.Error(t, err)
}

func TestFS_ParseError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write(t, dir, "bad.tsvt", "=sum(\n")
	ld := loader.FS(loader.Dir(dir), tsvsheet.DefaultLimits())

	_, _, err := ld("main.tsvt", "bad.tsvt")
	require.Error(t, err)
}

// ---- Unconfined (any path) ----------------------------------------------

func TestUnconfined_BareResolvesAgainstRoot(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write(t, dir, "child.tsvt", "=output(9)\n")
	ld := loader.Unconfined(loader.Dir(dir), tsvsheet.DefaultLimits())

	sub, resolved, err := ld("main.tsvt", "child.tsvt")
	require.NoError(t, err)
	assert.Equal(t, tsvsheet.Path(filepath.Join(dir, "child.tsvt")), resolved)
	assert.Equal(t, "9", sub.Compute()[0][0])
}

func TestUnconfined_RelativeFromSubdirBase(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write(t, dir, "sub/leaf.tsvt", "=output(7)\n")
	ld := loader.Unconfined(loader.Dir(dir), tsvsheet.DefaultLimits())

	_, resolved, err := ld(tsvsheet.Path(filepath.Join(dir, "sub", "mid.tsvt")), "leaf.tsvt")
	require.NoError(t, err)
	assert.Equal(t, tsvsheet.Path(filepath.Join(dir, "sub", "leaf.tsvt")), resolved)
}

func TestUnconfined_AbsoluteAndEscapeAllowed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	abs := write(t, dir, "abs.tsvt", "=output(1)\n")
	ld := loader.Unconfined(loader.Dir(t.TempDir()), tsvsheet.DefaultLimits()) // an unrelated root

	_, resolved, err := ld("main.tsvt", tsvsheet.Path(abs))
	require.NoError(t, err)
	assert.Equal(t, tsvsheet.Path(abs), resolved)
}

func TestUnconfined_MissingFile(t *testing.T) {
	t.Parallel()

	ld := loader.Unconfined(loader.Dir(t.TempDir()), tsvsheet.DefaultLimits())
	_, _, err := ld("main.tsvt", "absent.tsvt")
	require.Error(t, err)
}

// TestLoader_RefusesOverBudgetSiblings pins the 018 side door: a referenced
// sheet pays the same resident budget as the sheet that names it — the
// adversary's reproduction, where a one-cell sheet referencing an over-budget
// sibling materialized what the direct load refuses. Both modes refuse with
// ErrDocTooLarge naming the sibling; a raised budget loads it.
func TestLoader_RefusesOverBudgetSiblings(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "big.tsvt"), []byte("1\t2\n3\t4\n5\t6\n"), 0o600))
	tight := tsvsheet.DefaultLimits()
	tight.ResidentCells = 2

	for name, ld := range map[string]tsvsheet.Loader{
		"fs":         loader.FS(loader.Dir(dir), tight),
		"unconfined": loader.Unconfined(loader.Dir(dir), tight),
	} {
		_, _, err := ld("a.tsvt", "big.tsvt")
		require.ErrorIs(t, err, tsvsheet.ErrDocTooLarge, "%s must refuse the over-budget sibling", name)
		assert.Contains(t, err.Error(), "big.tsvt", "%s names the refused sheet", name)
	}

	raised := tight
	raised.ResidentCells = 6
	sheet, _, err := loader.FS(loader.Dir(dir), raised)("a.tsvt", "big.tsvt")
	require.NoError(t, err, "the same sibling under a raised budget loads")
	assert.Equal(t, tsvsheet.Grid{{"1", "2"}, {"3", "4"}, {"5", "6"}}, sheet.Source())
}
