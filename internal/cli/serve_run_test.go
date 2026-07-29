package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
	"github.com/tsvsheet/tsvsheet.go/internal/importer"
	"github.com/tsvsheet/tsvsheet.go/internal/session"
)

func TestBuildRefresh(t *testing.T) {
	t.Parallel()

	plain, err := session.New([]byte(sampleSheet)) // no clock functions
	require.NoError(t, err)
	volatile, err := session.New([]byte("=volatile(now())\n"))
	require.NoError(t, err)

	// An explicit interval wins.
	fixed, err := buildRefresh("5s", plain)
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, fixed(time.Now()))

	// Unset + volatile → default; unset + plain → off (nil).
	def, err := buildRefresh("", volatile)
	require.NoError(t, err)
	assert.Equal(t, defaultRefresh, def(time.Now()))

	off, err := buildRefresh("", plain)
	require.NoError(t, err)
	assert.Nil(t, off)

	// A malformed isnow pattern is rejected.
	_, err = buildRefresh("garbage!!!", plain)
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrInvalidValue)
}

func TestLoadServer_BadRefreshPattern(t *testing.T) {
	t.Parallel()

	_, err := loadServer(serveConfig{source: sheetFile(t), refresh: "garbage!!!"})
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrInvalidValue)
}

// sheetFile writes the sample spreadsheet to a temp file and returns its path.
func sheetFile(t *testing.T) sourcePath {
	t.Helper()
	return sourcePath(writeTemp(t, "s.tsvt", sampleSheet))
}

func TestLoadServer_OK(t *testing.T) {
	t.Parallel()

	srv, err := loadServer(serveConfig{source: sheetFile(t)})
	require.NoError(t, err)
	assert.NotNil(t, srv)
}

func TestLoadServer_RequiresFile(t *testing.T) {
	t.Parallel()

	_, err := loadServer(serveConfig{source: "-"})
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrInvalidValue)
}

func TestLoadServer_FileMissing(t *testing.T) {
	t.Parallel()

	_, err := loadServer(serveConfig{source: "/no/such.tsvt"})
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrOpenFile)
}

func TestLoadServer_SyntaxError(t *testing.T) {
	t.Parallel()

	path := writeTemp(t, "bad.tsvt", "1\t=sum(\n")
	_, err := loadServer(serveConfig{source: sourcePath(path)})
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrSyntax)
}

func TestSaver_WritesFile(t *testing.T) {
	t.Parallel()

	source := sheetFile(t)
	sess, _, err := loadEditable(source, false, tsvsheet.DefaultLimits(), nil)
	require.NoError(t, err)

	require.NoError(t, saver(sess, source)())

	written, err := os.ReadFile(string(source))
	require.NoError(t, err)
	assert.Equal(t, sampleSheet, string(written))
}

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

func TestRunServe_GracefulShutdown(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled → server starts then shuts down immediately

	err := runServe(ctx, serveConfig{source: sheetFile(t), host: defaultServeHost, port: 0})
	require.NoError(t, err)
}

func TestRunServe_LoadError(t *testing.T) {
	t.Parallel()

	err := runServe(context.Background(), serveConfig{source: "-"})
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrInvalidValue)
}

func TestRunServe_NonLoopbackWarns(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// A non-loopback host logs a warning, then shuts down on the cancelled ctx.
	err := runServe(ctx, serveConfig{source: sheetFile(t), host: nonLoopbackHost, port: 0})
	require.NoError(t, err)
}

func TestServeCommand_Integration(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := serveCommand()
	err := cmd.Run(ctx, []string{cmdServe, string(sheetFile(t)), "--port", "0"})
	require.NoError(t, err)
}

// nonLoopbackHost is a bind address that is not loopback, exercising the
// exposure guard and warning paths.
const nonLoopbackHost = "0.0.0.0"

// importFetcher builds a real (unused-at-rest) import Fetcher/Cache for the
// exposure tests — its mere presence flips serve's import posture.
func importFetcher() *importer.Cache {
	return importer.NewCache(importer.New(importer.Config{}))
}

func TestGuardImportExposure(t *testing.T) {
	t.Parallel()

	cache := importFetcher()

	// Imports on + a non-loopback bind → refused.
	err := guardImportExposure(serveConfig{host: nonLoopbackHost, fetcher: cache}, loopbackBind(false))
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrImportServeExposed)

	// Imports on + loopback → allowed.
	require.NoError(t, guardImportExposure(serveConfig{host: defaultServeHost, fetcher: cache}, loopbackBind(true)))

	// Imports off (nil fetcher) + non-loopback → allowed (only the file-serving
	// warning applies there, not the import refusal).
	require.NoError(t, guardImportExposure(serveConfig{host: nonLoopbackHost}, loopbackBind(false)))
}

func TestRunServe_ImportsOnNonLoopbackRefused(t *testing.T) {
	t.Parallel()

	cache := importFetcher()
	err := runServe(
		context.Background(),
		serveConfig{source: sheetFile(t), host: nonLoopbackHost, port: 0, fetcher: cache, cache: cache},
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrImportServeExposed)
}

func TestRunServe_ImportsOnLoopbackStarts(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cache := importFetcher()
	err := runServe(
		ctx,
		serveConfig{source: sheetFile(t), host: defaultServeHost, port: 0, fetcher: cache, cache: cache},
	)
	require.NoError(t, err)
}

func TestServeCommand_AllowImportRequiresHost(t *testing.T) {
	t.Parallel()

	cmd := serveCommand()
	err := cmd.Run(context.Background(), []string{cmdServe, "--allow-import", string(sheetFile(t)), "--port", "0"})
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrInvalidValue)
}

func TestServeCommand_AllowImportWithHost(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := serveCommand()
	err := cmd.Run(
		ctx,
		[]string{cmdServe, "--allow-import", "--import-host", "example.com", string(sheetFile(t)), "--port", "0"},
	)
	require.NoError(t, err)
}

func TestBuildRefresh_ScheduledCadence(t *testing.T) {
	t.Parallel()

	// A volatile() carrying its own isnow/duration cadence drives that cadence.
	scheduled, err := session.New([]byte("=volatile(rand(), \"5m\")\n"))
	require.NoError(t, err)
	next, err := buildRefresh("", scheduled)
	require.NoError(t, err)
	assert.Equal(t, 5*time.Minute, next(time.Now()))
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

	err := saveAtomic(filepath.Join(t.TempDir(), "absent"), "s.tsvt", []byte("1\n"))
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

	err := saveAtomic(dir, "s.tsvt", []byte("replacement\n"))
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

	err := saveAtomic(dir, "s.tsvt", []byte("replacement\n"))
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
