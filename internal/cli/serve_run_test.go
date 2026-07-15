package cli

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/constants"
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
	assert.ErrorIs(t, err, constants.ErrInvalidValue)
}

func TestLoadServer_BadRefreshPattern(t *testing.T) {
	t.Parallel()

	_, err := loadServer(serveConfig{source: sheetFile(t), refresh: "garbage!!!"})
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrInvalidValue)
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
	assert.ErrorIs(t, err, constants.ErrInvalidValue)
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
	assert.ErrorIs(t, err, constants.ErrSyntax)
}

func TestSaver_WritesFile(t *testing.T) {
	t.Parallel()

	source := sheetFile(t)
	sess, _, err := loadEditable(source, false)
	require.NoError(t, err)

	require.NoError(t, saver(sess, source)())

	written, err := os.ReadFile(string(source))
	require.NoError(t, err)
	assert.Equal(t, sampleSheet, string(written))
}

func TestSaver_WriteError(t *testing.T) {
	t.Parallel()

	source := sheetFile(t)
	sess, _, err := loadEditable(source, false)
	require.NoError(t, err)

	// A directory path cannot be written as a file.
	err = saver(sess, sourcePath(t.TempDir()))()
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrWriteFile)
}

func TestRunServe_GracefulShutdown(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled → server starts then shuts down immediately

	err := runServe(ctx, serveConfig{source: sheetFile(t), host: "127.0.0.1", port: 0})
	require.NoError(t, err)
}

func TestRunServe_LoadError(t *testing.T) {
	t.Parallel()

	err := runServe(context.Background(), serveConfig{source: "-"})
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrInvalidValue)
}

func TestServeCommand_Integration(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := serveCommand()
	err := cmd.Run(ctx, []string{cmdServe, string(sheetFile(t)), "--port", "0"})
	require.NoError(t, err)
}
