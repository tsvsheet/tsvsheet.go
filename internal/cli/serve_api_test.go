package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

// withServeStub captures the server the command builds instead of binding a
// socket, so the whole composition is exercised without a listener.
func withServeStub(t *testing.T) *[]*http.Server {
	t.Helper()
	var built []*http.Server
	prev := listenAndServe
	listenAndServe = func(_ context.Context, server *http.Server) error {
		built = append(built, server)
		return nil
	}
	t.Cleanup(func() { listenAndServe = prev })
	return &built
}

// apiRoot is a directory holding one document.
func apiRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.tsvt"), []byte("2\t=A1*3\n"), 0o600))
	return dir
}

func TestServeAPIServesTheConfiguredRoot(t *testing.T) {
	built := withServeStub(t)
	_, err := runCLI(t, cmdServe, cmdServeAPI, "--root", apiRoot(t), "--addr", "127.0.0.1:0")
	require.NoError(t, err)
	require.Len(t, *built, 1)
	server := (*built)[0]
	assert.Equal(t, "127.0.0.1:0", server.Addr)

	source := httptest.NewRecorder()
	server.Handler.ServeHTTP(source, httptest.NewRequest(http.MethodGet, "/a.tsvt", nil))
	assert.Equal(t, http.StatusOK, source.Code)
	assert.Equal(t, "2\t=A1*3\n", source.Body.String(), "the source is served with its formula intact")
	assert.NotEmpty(t, source.Header().Get("ETag"))

	computed := httptest.NewRecorder()
	server.Handler.ServeHTTP(computed, httptest.NewRequest(http.MethodGet, "/a.tsvt!B1", nil))
	assert.Equal(t, "6\n", computed.Body.String(), "and the compute plane answers a reference read")
}

func TestServeAPIWithoutComputeServesTheDocumentPlaneOnly(t *testing.T) {
	built := withServeStub(t)
	_, err := runCLI(t, cmdServe, cmdServeAPI, "--root", apiRoot(t), "--no-compute")
	require.NoError(t, err)
	require.Len(t, *built, 1)
	rec := httptest.NewRecorder()
	(*built)[0].Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/a.tsvt", nil))
	assert.Equal(t, "edits, events", rec.Header().Get("Tsvsheet-Capabilities"))
}

// TestServeAPIRefusesExposure pins that a non-loopback bind is an error rather
// than a warning: this server carries no TLS and no authentication, and a
// warning is read once while a bind lasts until the process ends.
func TestServeAPIRefusesExposure(t *testing.T) {
	_ = withServeStub(t)
	for _, exposed := range []string{"0.0.0.0:8787", "example.com:80", "[::]:8787"} {
		_, err := runCLI(t, cmdServe, cmdServeAPI, "--root", apiRoot(t), "--addr", exposed)
		require.Error(t, err, exposed)
		assert.ErrorIs(t, err, constants.ErrServeExposed, exposed)
	}
}

func TestServeAPIRefusesAMalformedAddress(t *testing.T) {
	_ = withServeStub(t)
	_, err := runCLI(t, cmdServe, cmdServeAPI, "--root", apiRoot(t), "--addr", "no-port")
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrServeAddress)
}

func TestServeAPIAcceptsEveryLoopbackForm(t *testing.T) {
	_ = withServeStub(t)
	for _, loopback := range []string{"127.0.0.1:0", "localhost:0", "[::1]:0"} {
		_, err := runCLI(t, cmdServe, cmdServeAPI, "--root", apiRoot(t), "--addr", loopback)
		assert.NoError(t, err, loopback)
	}
}

func TestServeAPIRefusesAMissingRoot(t *testing.T) {
	_ = withServeStub(t)
	_, err := runCLI(t, cmdServe, cmdServeAPI, "--root", filepath.Join(t.TempDir(), "absent"))
	require.Error(t, err)
}

// TestServeAPIHonoursTheGlobalCellCap pins that --max-cells reaches the served
// engine: an edit beyond the capped grid is refused rather than applied.
func TestServeAPIHonoursTheGlobalCellCap(t *testing.T) {
	built := withServeStub(t)
	_, err := runCLI(t, "--max-cells", "2", cmdServe, cmdServeAPI, "--root", apiRoot(t))
	require.NoError(t, err)
	require.Len(t, *built, 1)

	rec := httptest.NewRecorder()
	head := httptest.NewRecorder()
	(*built)[0].Handler.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/a.tsvt", nil))
	req := httptest.NewRequest(http.MethodPost, "/a.tsvt", strings.NewReader("setCell\tE9\tfar\n"))
	req.Header.Set("Content-Type", "application/vnd.tsvsheet.edits+tsv")
	req.Header.Set("If-Match", head.Header().Get("ETag"))
	(*built)[0].Handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

// TestServeAPIWithoutARootIsAUsageMistake pins the exit code contract: a
// missing required input prints the command's own help and exits 2, the same
// as every other command. urfave's Required flag would exit 1 with a bare
// message, which this repo reserves for a runtime failure.
func TestServeAPIWithoutARootIsAUsageMistake(t *testing.T) {
	_ = withServeStub(t)
	out, err := runCLI(t, cmdServe, cmdServeAPI)
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrUsage)
	assert.Equal(t, exitSyntaxError, exitCode(err))
	assert.Contains(t, out, "USAGE:")
	assert.Contains(t, out, cmdServeAPI)
}

// TestServeAPIShutsDownOnContextCancellation pins that an interrupted server
// drains rather than dying: a request dropped mid-write leaves a client unable
// to tell a refused edit from a lost one, which is exactly the distinction the
// conditional-write discipline exists to preserve.
func TestServeAPIShutsDownOnContextCancellation(t *testing.T) {
	prev := listenAndServe
	t.Cleanup(func() { listenAndServe = prev })
	listenAndServe = prev

	server := &http.Server{Addr: "127.0.0.1:0", Handler: http.NewServeMux(), ReadHeaderTimeout: apiHeaderTimeout}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- listenAndServe(ctx, server) }()
	cancel()
	select {
	case err := <-done:
		assert.NoError(t, err, "a cancelled serve shuts down cleanly")
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after its context was cancelled")
	}
}

// TestAPIHeaderTimeoutIsSet pins the bound on how long a client may dawdle
// over its request head: without it a stalled peer holds a connection open
// indefinitely, and nothing else in the composition would notice.
func TestAPIHeaderTimeoutIsSet(t *testing.T) {
	built := withServeStub(t)
	_, err := runCLI(t, cmdServe, cmdServeAPI, "--root", apiRoot(t))
	require.NoError(t, err)
	require.Len(t, *built, 1)
	assert.Equal(t, apiHeaderTimeout, (*built)[0].ReadHeaderTimeout)
	assert.Positive(t, (*built)[0].ReadHeaderTimeout)
}

// TestServeAPIDefaultAddress pins the address a bare `serve api` binds, port
// included: an operator who omits --addr and a client that assumes the default
// must agree, and a changed port is a silent connection refusal.
func TestServeAPIDefaultAddress(t *testing.T) {
	built := withServeStub(t)
	_, err := runCLI(t, cmdServe, cmdServeAPI, "--root", apiRoot(t))
	require.NoError(t, err)
	require.Len(t, *built, 1)
	assert.Equal(t, "127.0.0.1:8787", (*built)[0].Addr)
}
