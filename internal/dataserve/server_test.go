package dataserve_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
	"github.com/tsvsheet/tsvsheet.go/internal/dataserve"
)

// dataRoot builds a published directory with a nested file, so both the flat
// and the nested reference shapes are exercised against a real filesystem.
func dataRoot(t *testing.T) dataserve.Root {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "balances.tsv"), []byte("Brokerage\t310000\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "rates.csv"), []byte("a,b\n1,2\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("secret\n"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "macro"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "macro", "nfci.tsv"), []byte("2026-07-24\t-0.41\n"), 0o600))
	return dataserve.Root(dir)
}

// get issues a request against the handler and returns the response.
func get(t *testing.T, root dataserve.Root, target string) *http.Response {
	t.Helper()

	rec := httptest.NewRecorder()
	handler, err := dataserve.Handler(root)
	require.NoError(t, err)
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec.Result()
}

func TestHandler_ServesTabularFilesWithTheirMediaType(t *testing.T) {
	t.Parallel()

	root := dataRoot(t)
	cases := map[string]struct{ media, body string }{
		"/balances.tsv":   {"text/tab-separated-values", "Brokerage\t310000\n"},
		"/rates.csv":      {"text/csv", "a,b\n1,2\n"},
		"/macro/nfci.tsv": {"text/tab-separated-values", "2026-07-24\t-0.41\n"},
	}
	for target, want := range cases {
		resp := get(t, root, target)
		defer func() { _ = resp.Body.Close() }()

		require.Equal(t, http.StatusOK, resp.StatusCode, target)
		assert.Equal(t, want.media, resp.Header.Get("Content-Type"), target)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, want.body, string(body), target)
	}
}

func TestHandler_RefusesUnpublishedExtensions(t *testing.T) {
	t.Parallel()

	root := dataRoot(t)
	// A file that exists but is not tabular must not be readable: the server
	// publishes data, not the directory it was pointed at.
	for _, target := range []string{"/notes.txt", "/", "/macro", "/balances"} {
		resp := get(t, root, target)
		_ = resp.Body.Close()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, target)
	}
}

func TestHandler_ExtensionMatchIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "X.TSV"), []byte("a\n"), 0o600))

	resp := get(t, dataserve.Root(dir), "/X.TSV")
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/tab-separated-values", resp.Header.Get("Content-Type"))
}

func TestHandler_TraversalCannotEscapeTheRoot(t *testing.T) {
	t.Parallel()

	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.tsv"), []byte("leaked\n"), 0o600))
	root := dataserve.Root(t.TempDir())

	// Independent of the importer's own confinement: a client that is not the
	// importer must not be able to climb out either.
	for _, target := range []string{
		"/../secret.tsv",
		"/../../etc/hosts.tsv",
		"/macro/../../secret.tsv",
	} {
		resp := get(t, root, target)
		_ = resp.Body.Close()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, target)
	}
}

func TestHandler_RefusesNonGetMethods(t *testing.T) {
	t.Parallel()

	root := dataRoot(t)
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodHead} {
		rec := httptest.NewRecorder()
		handler, err := dataserve.Handler(root)
		require.NoError(t, err)
		handler.ServeHTTP(rec, httptest.NewRequest(method, "/balances.tsv", nil))
		assert.Equal(t, http.StatusMethodNotAllowed, rec.Result().StatusCode, method)
	}
}

func TestHandler_RefusesADirectoryShapedLikeAFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A directory named like a data file has nothing to stream.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "shaped.tsv"), 0o700))

	resp := get(t, dataserve.Root(dir), "/shaped.tsv")
	_ = resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandler_MissingRootFailsAtConstruction(t *testing.T) {
	t.Parallel()

	// An operator who mistypes --data learns now, not once per reference.
	_, err := dataserve.Handler(dataserve.Root(filepath.Join(t.TempDir(), "absent")))
	require.ErrorIs(t, err, constants.ErrDataRoot)
}

func TestStart_MissingRootFailsBeforeBinding(t *testing.T) {
	t.Parallel()

	_, err := dataserve.Start(dataserve.Root(filepath.Join(t.TempDir(), "absent")), dataserve.LoopbackAny)
	require.ErrorIs(t, err, constants.ErrDataRoot)
}

func TestStart_ServesOverLoopbackAndClosesCleanly(t *testing.T) {
	t.Parallel()

	server, err := dataserve.Start(dataRoot(t), dataserve.LoopbackAny)
	require.NoError(t, err)

	assert.Regexp(t, `^http://127\.0\.0\.1:\d+/$`, server.Base(), "the base carries the assigned port")
	assert.Equal(t, "http://"+server.Addr()+"/", server.Base(), "Addr and Base agree on the bound port")

	resp, err := http.Get(server.Base() + "balances.tsv") //nolint:noctx // a loopback probe in a test
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, "Brokerage\t310000\n", string(body))

	require.NoError(t, server.Close())

	// After Close the listener is gone — nothing outlives the command.
	after, err := http.Get(server.Base() + "balances.tsv") //nolint:noctx // a loopback probe in a test
	if after != nil {
		_ = after.Body.Close()
	}
	require.Error(t, err)
}

func TestStart_UnbindableAddress(t *testing.T) {
	t.Parallel()

	_, err := dataserve.Start(dataRoot(t), "127.0.0.1:-1")
	require.ErrorIs(t, err, constants.ErrDataListen)
}

func TestClose_ZeroServerIsSafe(t *testing.T) {
	t.Parallel()

	// A command that never started a server still defers Close.
	require.NoError(t, dataserve.Server{}.Close())
}

func TestHandler_RootRemovedAfterConstructionIsServerError(t *testing.T) {
	t.Parallel()

	// The startup check is not a cached capability: the directory can go away
	// while the server runs, and a request must fail rather than panic.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "balances.tsv"), []byte("a\n"), 0o600))
	handler, err := dataserve.Handler(dataserve.Root(dir))
	require.NoError(t, err)
	require.NoError(t, os.RemoveAll(dir))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/balances.tsv", nil))
	assert.Equal(t, http.StatusInternalServerError, rec.Result().StatusCode)
}
