package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/constants"
)

// worksheetFiles writes a template and data file and returns their paths.
func worksheetFiles(t *testing.T) (tmpl, data sourcePath) {
	t.Helper()
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "w.tsvt")
	dataPath := filepath.Join(dir, "w.tsv")
	require.NoError(t, os.WriteFile(tmplPath, []byte(sampleTemplate), 0o600))
	require.NoError(t, os.WriteFile(dataPath, []byte(sampleData), 0o600))
	return sourcePath(tmplPath), sourcePath(dataPath)
}

func TestLoadServer_OK(t *testing.T) {
	t.Parallel()

	tmpl, data := worksheetFiles(t)
	srv, err := loadServer(tmpl, data)
	require.NoError(t, err)
	assert.NotNil(t, srv)
}

func TestLoadServer_RequiresFiles(t *testing.T) {
	t.Parallel()

	_, err := loadServer("-", "-")
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrInvalidValue)
}

func TestLoadServer_TemplateMissing(t *testing.T) {
	t.Parallel()

	_, data := worksheetFiles(t)
	_, err := loadServer("/no/such.tsvt", data)
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrOpenFile)
}

func TestLoadServer_DataMissing(t *testing.T) {
	t.Parallel()

	tmpl, _ := worksheetFiles(t)
	_, err := loadServer(tmpl, "/no/such.tsv")
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrOpenFile)
}

func TestLoadServer_SyntaxError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "bad.tsvt")
	dataPath := filepath.Join(dir, "w.tsv")
	require.NoError(t, os.WriteFile(tmplPath, []byte("=sum("), 0o600))
	require.NoError(t, os.WriteFile(dataPath, []byte(sampleData), 0o600))

	_, err := loadServer(sourcePath(tmplPath), sourcePath(dataPath))
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrSyntax)
}

func TestLoadSession_DataReadError(t *testing.T) {
	t.Parallel()

	// A data file that exceeds the scanner's line bound surfaces a read error.
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "w.tsvt")
	dataPath := filepath.Join(dir, "big.tsv")
	require.NoError(t, os.WriteFile(tmplPath, []byte(sampleTemplate), 0o600))
	big := make([]byte, 2<<20) // 2 MiB, no newline → exceeds maxLineBytes
	for i := range big {
		big[i] = 'x'
	}
	require.NoError(t, os.WriteFile(dataPath, big, 0o600))

	_, err := loadSession(sourcePath(tmplPath), sourcePath(dataPath))
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrReadInput)
}

func TestSaver_WritesFiles(t *testing.T) {
	t.Parallel()

	tmpl, data := worksheetFiles(t)
	srv, err := loadServer(tmpl, data)
	require.NoError(t, err)
	require.NotNil(t, srv)

	// Reach the saver through the server's save endpoint would need the session;
	// instead build the saver directly against a fresh session.
	sess, err := loadSession(tmpl, data)
	require.NoError(t, err)
	require.NoError(t, saver(sess, tmpl, data)())

	written, err := os.ReadFile(string(tmpl))
	require.NoError(t, err)
	assert.Equal(t, []byte(sampleTemplate), written)
}

func TestSaver_TemplateWriteError(t *testing.T) {
	t.Parallel()

	tmpl, data := worksheetFiles(t)
	sess, err := loadSession(tmpl, data)
	require.NoError(t, err)

	// A directory path cannot be written as a file.
	err = saver(sess, sourcePath(t.TempDir()), data)()
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrWriteFile)
}

func TestSaver_DataWriteError(t *testing.T) {
	t.Parallel()

	tmpl, data := worksheetFiles(t)
	sess, err := loadSession(tmpl, data)
	require.NoError(t, err)

	err = saver(sess, tmpl, sourcePath(t.TempDir()))()
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrWriteFile)
}

func TestRunServe_GracefulShutdown(t *testing.T) {
	t.Parallel()

	tmpl, data := worksheetFiles(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled → server starts then shuts down immediately

	err := runServe(ctx, serveConfig{template: tmpl, data: data, host: "127.0.0.1", port: 0})
	require.NoError(t, err)
}

func TestRunServe_LoadError(t *testing.T) {
	t.Parallel()

	err := runServe(context.Background(), serveConfig{template: "-", data: "-"})
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrInvalidValue)
}

func TestServeCommand_Integration(t *testing.T) {
	t.Parallel()

	tmpl, data := worksheetFiles(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := serveCommand()
	err := cmd.Run(ctx, []string{"serve", "--template", string(tmpl), "--data", string(data), "--port", "0"})
	require.NoError(t, err)
}
