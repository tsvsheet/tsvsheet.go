package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"
)

func TestSaver_WriteError(t *testing.T) {
	t.Parallel()

	source := sheetFile(t)
	sess, _, err := loadEditable(source, false, tsvsheet.DefaultLimits(), nil)
	require.NoError(t, err)

	// A directory path cannot be written as a file.
	err = saver(sess, sourcePath(t.TempDir()))()
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrWriteFile)
}

// TestSaver_ConfinedAtomicWrite proves the save contract: the bytes land in the
// sheet, and no staging file is left behind.
func TestSaver_ConfinedAtomicWrite(t *testing.T) {
	t.Parallel()

	source := sheetFile(t)
	sess, _, err := loadEditable(source, false, tsvsheet.DefaultLimits(), nil)
	require.NoError(t, err)

	require.NoError(t, saver(sess, source)())

	saved, err := os.ReadFile(string(source))
	require.NoError(t, err)
	assert.Equal(t, string(sess.Source()), string(saved))

	_, err = os.Stat(string(source) + tempSuffix)
	assert.True(t, os.IsNotExist(err), "the staging file is renamed away, never left behind")
}

// TestSaver_SymlinkOutOfTheDirectoryIsRefused is the confinement contract: a
// sheet path that is a symlink pointing outside its own directory must not be
// usable to overwrite the file it points at.
func TestSaver_SymlinkOutOfTheDirectoryIsRefused(t *testing.T) {
	t.Parallel()

	victimDir := t.TempDir()
	victim := filepath.Join(victimDir, "precious")
	require.NoError(t, os.WriteFile(victim, []byte("do not clobber\n"), 0o600))

	sheetDir := t.TempDir()
	link := filepath.Join(sheetDir, "sheet.tsvt")
	require.NoError(t, os.Symlink(victim, link))

	sess, _, err := loadEditable(sheetFile(t), false, tsvsheet.DefaultLimits(), nil)
	require.NoError(t, err)

	// The rename targets the link name inside the confined root; the victim
	// outside the root must be untouched either way.
	_ = saver(sess, sourcePath(link))()

	after, err := os.ReadFile(victim)
	require.NoError(t, err)
	assert.Equal(t, "do not clobber\n", string(after), "the symlink target was not overwritten")
}

func TestSaveAtomic_MissingDirectory(t *testing.T) {
	t.Parallel()

	err := saveAtomic(sheetDir(filepath.Join(t.TempDir(), "absent")), "s.tsvt", []byte("1\n"))
	require.ErrorIs(t, err, tsvsheet.ErrWriteFile)
}

// TestSaveAtomic_WriteFailureLeavesTheSheetIntact stubs the staging write. These
// stubs are process-global, so this test and the rename one below are not
// parallel.
func TestSaveAtomic_WriteFailureLeavesTheSheetIntact(t *testing.T) {
	dir := t.TempDir()
	sheet := filepath.Join(dir, "s.tsvt")
	require.NoError(t, os.WriteFile(sheet, []byte("original\n"), 0o600))

	prev := writeFileIn
	writeFileIn = func(*os.Root, string, []byte, os.FileMode) error { return errors.New("disk full") }
	t.Cleanup(func() { writeFileIn = prev })

	err := saveAtomic(sheetDir(dir), "s.tsvt", []byte("replacement\n"))
	require.ErrorIs(t, err, tsvsheet.ErrWriteFile)

	kept, err := os.ReadFile(sheet)
	require.NoError(t, err)
	assert.Equal(t, "original\n", string(kept), "a failed save must not damage the sheet")
}

func TestSaveAtomic_RenameFailureLeavesTheSheetIntactAndRemovesTheStagingFile(t *testing.T) {
	dir := t.TempDir()
	sheet := filepath.Join(dir, "s.tsvt")
	require.NoError(t, os.WriteFile(sheet, []byte("original\n"), 0o600))

	prev := renameIn
	renameIn = func(*os.Root, string, string) error { return errors.New("cross-device link") }
	t.Cleanup(func() { renameIn = prev })

	err := saveAtomic(sheetDir(dir), "s.tsvt", []byte("replacement\n"))
	require.ErrorIs(t, err, tsvsheet.ErrWriteFile)

	kept, err := os.ReadFile(sheet)
	require.NoError(t, err)
	assert.Equal(t, "original\n", string(kept), "a failed rename must not damage the sheet")

	// The staging name carries a process and counter suffix, so asserting on
	// `sheet + tempSuffix` would be checking a path this code never writes —
	// trivially absent, and blind to a staging file that really survived.
	leftovers, globErr := filepath.Glob(sheet + tempSuffix + "*")
	require.NoError(t, globErr)
	assert.Empty(t, leftovers, "no staging file survives a failed rename")
}

// TestSaver_BareFilenameSavesInTheWorkingDirectory covers the no-directory
// case: `tsv serve sheet.tsvt` splits to an empty dir, which must become ".".
func TestSaver_BareFilenameSavesInTheWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bare.tsvt"), []byte("1\t2\n"), 0o600))
	t.Chdir(dir)

	sess, _, err := loadEditable(sourcePath("bare.tsvt"), false, tsvsheet.DefaultLimits(), nil)
	require.NoError(t, err)
	require.NoError(t, saver(sess, sourcePath("bare.tsvt"))())

	saved, err := os.ReadFile(filepath.Join(dir, "bare.tsvt"))
	require.NoError(t, err)
	assert.Equal(t, "1\t2\n", string(saved))
}

// TestWriteFileIn_StagesSoTheReplacementIsAtomic pins the "atomic" the save
// path claims. The staging write goes to a separate name and only a rename
// publishes it, so the sheet is never observed half-written: at every instant
// the sheet's own path holds either the old bytes or the new ones, never a
// truncated prefix of either.
func TestWriteFileIn_StagesSoTheReplacementIsAtomic(t *testing.T) {
	dir := t.TempDir()
	sheet := filepath.Join(dir, "s.tsvt")
	require.NoError(t, os.WriteFile(sheet, []byte("original\n"), 0o600))

	// Observe what the sheet holds at the moment the staging write completes:
	// still the original, because nothing has been renamed over it yet.
	var atStagingTime string
	prev := writeFileIn
	writeFileIn = func(root *os.Root, name string, data []byte, perm os.FileMode) error {
		err := prev(root, name, data, perm)
		seen, readErr := os.ReadFile(sheet)
		require.NoError(t, readErr)
		atStagingTime = string(seen)
		return err
	}
	t.Cleanup(func() { writeFileIn = prev })

	require.NoError(t, saveAtomic(sheetDir(dir), "s.tsvt", []byte("replacement\n")))

	assert.Equal(t, "original\n", atStagingTime, "the sheet is untouched until the rename")

	after, err := os.ReadFile(sheet)
	require.NoError(t, err)
	assert.Equal(t, "replacement\n", string(after), "and fully replaced after it")
}

// brokenReader always fails, driving the edits read-error path.
type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, errors.New("broken pipe") }

func TestRunApply_EditsReadFailure(t *testing.T) {
	t.Parallel()

	sheet := writeSheet(t, "1\t2\n")
	streams := Streams{In: brokenReader{}, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := runApply(streams, sourcePath(sheet), "-", tsvsheet.DefaultLimits(), applyWrite)
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrReadInput)
}

func TestRunApply_WriteFailureLeavesSheetIntact(t *testing.T) {
	// Serial: swaps the process-global writeFileIn stub.
	sheet := writeSheet(t, "1\t2\n")
	edits := writeEdits(t, "setCell\tB1\t9\n")
	prev := writeFileIn
	writeFileIn = func(*os.Root, string, []byte, os.FileMode) error { return errors.New("disk full") }
	t.Cleanup(func() { writeFileIn = prev })
	streams, _, _ := streamsWith("")
	err := runApply(streams, sourcePath(sheet), sourcePath(edits), tsvsheet.DefaultLimits(), applyWrite)
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrWriteFile)
	saved, readErr := os.ReadFile(sheet)
	require.NoError(t, readErr)
	assert.Equal(t, "1\t2\n", string(saved))
}

// TestRunApply_PreservesTheSheetFileMode pins that an edit is an edit, not a
// permission change: a world-readable sheet stays world-readable.
func TestRunApply_PreservesTheSheetFileMode(t *testing.T) {
	sheet := writeSheet(t, "1\t2\n")
	require.NoError(t, os.Chmod(sheet, 0o644))
	edits := writeEdits(t, "setCell\tB1\t9\n")
	streams, _, _ := streamsWith("")
	require.NoError(t, runApply(streams, sourcePath(sheet), sourcePath(edits), tsvsheet.DefaultLimits(), applyWrite))
	info, err := os.Stat(sheet)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}

func TestRunApply_NewSheetIsPrivate(t *testing.T) {
	// A sheet the save creates (no prior file) gets the private default.
	dir := t.TempDir()
	sheet := filepath.Join(dir, "fresh.tsvt")
	require.NoError(t, saveAtomic(sheetDir(dir), "fresh.tsvt", []byte("1\n")))
	info, err := os.Stat(sheet)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
