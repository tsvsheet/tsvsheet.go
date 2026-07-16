package cli

import (
	"bytes"
	"context"
	"io"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uplang/go-tsvsheet"
	"github.com/urfave/cli/v3"

	"github.com/uplang/tsvsheet.go/internal/importer"
	"github.com/uplang/tsvsheet.go/internal/session"
)

// runImportFlags parses args against a throwaway command carrying the import
// flags and returns what resolveImport makes of them.
func runImportFlags(t *testing.T, args ...string) (tsvsheet.Fetcher, *importer.Cache, error) {
	t.Helper()
	var (
		gotFetcher tsvsheet.Fetcher
		gotCache   *importer.Cache
		gotErr     error
	)
	cmd := &cli.Command{
		Name:  "x",
		Flags: importFlags(),
		Action: func(_ context.Context, c *cli.Command) error {
			gotFetcher, gotCache, gotErr = resolveImport(c)
			return nil
		},
	}
	require.NoError(t, cmd.Run(context.Background(), append([]string{"x"}, args...)))
	return gotFetcher, gotCache, gotErr
}

func TestResolveImport_OffByDefault(t *testing.T) {
	t.Parallel()

	// No --allow-import → imports disabled: a nil Fetcher and nil Cache, no error.
	fetcher, cache, err := runImportFlags(t)
	require.NoError(t, err)
	assert.Nil(t, fetcher)
	assert.Nil(t, cache)
}

func TestResolveImport_AllowWithoutHostErrors(t *testing.T) {
	t.Parallel()

	// --allow-import with an empty allowlist is a configuration error, not a
	// silent deny.
	_, _, err := runImportFlags(t, "--allow-import")
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrInvalidValue)
}

func TestResolveImport_AllowWithHostBuildsFetcher(t *testing.T) {
	t.Parallel()

	// --allow-import plus at least one host builds the Fetcher (and its refresh
	// cache); the Cache is itself the tsvsheet.Fetcher the engine consumes.
	fetcher, cache, err := runImportFlags(t, "--allow-import", "--import-host", "example.com")
	require.NoError(t, err)
	require.NotNil(t, cache)
	assert.Equal(t, tsvsheet.Fetcher(cache), fetcher)
}

func TestHostPatterns_Converts(t *testing.T) {
	t.Parallel()

	got := hostPatterns([]string{"example.com", "*.internal"})
	assert.Equal(t, []importer.HostPattern{"example.com", "*.internal"}, got)
}

func TestWireRefresh_NilCacheIsNoOp(t *testing.T) {
	t.Parallel()

	sess, err := session.New([]byte("1\n"))
	require.NoError(t, err)
	wireRefresh(sess, nil) // nil branch: registers nothing
	assert.NotPanics(t, func() { _ = sess.RefreshImports() })
}

func TestWireRefresh_RegistersCacheClear(t *testing.T) {
	t.Parallel()

	sess, err := session.New([]byte("1\n"))
	require.NoError(t, err)
	cache := importer.NewCache(importer.New(importer.Config{}))
	wireRefresh(sess, cache) // non-nil branch: routes RefreshImports through Clear
	assert.NotPanics(t, func() { _ = sess.RefreshImports() })
}

func TestComputeOptions_CarriesFetcher(t *testing.T) {
	t.Parallel()

	fetcher := tsvsheet.Fetcher(importer.New(importer.Config{}))

	// Both the stdin branch and the file branch carry the injected Fetcher.
	stdinOpts := computeOptions("-", false, tsvsheet.DefaultLimits(), fetcher)
	assert.Equal(t, fetcher, stdinOpts.Fetcher)

	fileOpts := computeOptions(sheetFile(t), false, tsvsheet.DefaultLimits(), fetcher)
	assert.Equal(t, fetcher, fileOpts.Fetcher)
}

func TestRenderCommand_AllowImportRequiresHost(t *testing.T) {
	t.Parallel()

	cmd := renderCommand()
	err := cmd.Run(context.Background(), []string{cmdRender, "--allow-import", string(sheetFile(t))})
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrInvalidValue)
}

func TestRenderCommand_AllowImportWithHost(t *testing.T) {
	t.Parallel()

	cmd := renderCommand()
	cmd.Writer = &bytes.Buffer{}
	err := cmd.Run(
		context.Background(),
		[]string{cmdRender, "--allow-import", "--import-host", "example.com", string(sheetFile(t))},
	)
	require.NoError(t, err)
}

func TestTUICommand_AllowImportRequiresHost(t *testing.T) {
	t.Parallel()

	cmd := tuiCommand()
	err := cmd.Run(context.Background(), []string{cmdTUI, "--allow-import", string(sheetFile(t))})
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrInvalidValue)
}

func TestTUICommand_AllowImportWithHost(t *testing.T) {
	withRunProgram(t, func(tea.Model, io.Reader, io.Writer) error { return nil })

	cmd := tuiCommand()
	err := cmd.Run(
		context.Background(),
		[]string{cmdTUI, "--allow-import", "--import-host", "example.com", string(sheetFile(t))},
	)
	require.NoError(t, err)
}
