package cli

import (
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

	_, statErr := os.Stat(sheet + tempSuffix)
	assert.True(t, os.IsNotExist(statErr), "the staging file is cleaned up on failure")
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
