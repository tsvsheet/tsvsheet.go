package cli

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uplang/go-tsvsheet"

	"github.com/uplang/tsvsheet.go/internal/constants"
	"github.com/uplang/tsvsheet.go/internal/importer"
	"github.com/uplang/tsvsheet.go/internal/session"
)

func TestBuildRefresh(t *testing.T) {
	t.Parallel()

	plain, err := session.New([]byte(sampleSheet)) // no clock functions
	require.NoError(t, err)
	volatile, err := session.New([]byte("=now()\n"))
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
