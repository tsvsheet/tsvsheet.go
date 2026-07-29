package importer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

const cellMedia = tsvsheet.MediaType("application/vnd.tsvsheet.cell+tsv")

// tlsFetcher stands up a TLS test server with handler h and returns a Fetcher
// wired to trust it, allowlisting the server's 127.0.0.1 host.
func tlsFetcher(t *testing.T, h http.HandlerFunc) (Fetcher, *httptest.Server) {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	cfg := Config{
		Client:       srv.Client(),
		AllowedHosts: []HostPattern{"127.0.0.1"},
		Timeout:      2 * time.Second,
		MaxBytes:     1024,
	}
	return New(cfg), srv
}

func TestFetch_HappyPathStripsCharsetParam(t *testing.T) {
	t.Parallel()

	f, srv := tlsFetcher(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", string(cellMedia)+"; charset=utf-8")
		_, _ = w.Write([]byte("42\n"))
	})

	res, err := f.Fetch(tsvsheet.ImportURL(srv.URL), cellMedia)
	require.NoError(t, err)
	assert.Equal(t, cellMedia, res.ContentType) // param stripped → exact match
	assert.Equal(t, "42\n", string(res.Body))
}

func TestFetch_GenericTabularTypesPassThroughNormalized(t *testing.T) {
	t.Parallel()

	// A standard tabular Content-Type reaches the engine as its normalized base
	// type — the engine's accept set, not the Fetcher, decides admissibility.
	for _, ct := range []string{"text/tab-separated-values; charset=utf-8", "text/csv"} {
		f, srv := tlsFetcher(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", ct)
			_, _ = w.Write([]byte("42\n"))
		})
		res, err := f.Fetch(tsvsheet.ImportURL(srv.URL), cellMedia)
		require.NoError(t, err)
		base, _, _ := strings.Cut(ct, ";")
		assert.Equal(t, tsvsheet.MediaType(base), res.ContentType, ct)
	}
}

func TestFetch_SendsAcceptHeader(t *testing.T) {
	t.Parallel()

	var got string
	f, srv := tlsFetcher(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Accept")
		w.Header().Set("Content-Type", string(cellMedia))
		_, _ = w.Write([]byte("x"))
	})

	_, err := f.Fetch(tsvsheet.ImportURL(srv.URL), cellMedia)
	require.NoError(t, err)
	// The header negotiates: vendor type preferred, standard tabular admitted.
	assert.Equal(t, string(cellMedia)+", text/tab-separated-values;q=0.9, text/csv;q=0.8", got)
}

func TestFetch_SchemeMustBeHTTPS(t *testing.T) {
	t.Parallel()

	f := New(Config{AllowedHosts: []HostPattern{"example.com"}})
	for _, raw := range []string{"http://example.com/x", "file:///etc/passwd", "ftp://example.com/x"} {
		_, err := f.Fetch(tsvsheet.ImportURL(raw), cellMedia)
		assert.ErrorIs(t, err, constants.ErrImportScheme, raw)
	}
}

func TestFetch_MalformedURL(t *testing.T) {
	t.Parallel()

	f := New(Config{AllowedHosts: []HostPattern{"example.com"}})
	// A DEL control character makes net/url (and thus NewRequestWithContext) reject the URL.
	_, err := f.Fetch(tsvsheet.ImportURL("https://example.com/\x7f"), cellMedia)
	assert.ErrorIs(t, err, constants.ErrImportURL)
}

func TestFetch_HostDenied_NilClientDefault(t *testing.T) {
	t.Parallel()

	// Empty allowlist denies everything; a nil Client exercises New's default-client branch.
	f := New(Config{})
	_, err := f.Fetch(tsvsheet.ImportURL("https://example.com/x"), cellMedia)
	assert.ErrorIs(t, err, constants.ErrImportHostDenied)
}

func TestFetch_NonAllowlistedHostDenied(t *testing.T) {
	t.Parallel()

	f := New(Config{AllowedHosts: []HostPattern{"good.example.com"}})
	_, err := f.Fetch(tsvsheet.ImportURL("https://evil.example.com/x"), cellMedia)
	assert.ErrorIs(t, err, constants.ErrImportHostDenied)
}

func TestFetch_Timeout(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond) // outlast the 20ms client deadline, then finish
		w.Header().Set("Content-Type", string(cellMedia))
	}))
	t.Cleanup(srv.Close)
	f := New(Config{
		AllowedHosts: []HostPattern{"127.0.0.1"},
		Timeout:      20 * time.Millisecond,
		MaxBytes:     1024,
		Client:       srv.Client(),
	})

	_, err := f.Fetch(tsvsheet.ImportURL(srv.URL), cellMedia)
	assert.ErrorIs(t, err, constants.ErrImportFetch)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// ---- redirects ----------------------------------------------------------

func TestFetch_LoopbackHTTPAllowed(t *testing.T) {
	t.Parallel()

	// A direct plain-http request to a loopback host is permitted: http is
	// allowed when the target is loopback.
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", string(cellMedia))
		_, _ = w.Write([]byte("local"))
	}))
	t.Cleanup(plain.Close)
	f := New(Config{
		AllowedHosts: []HostPattern{"127.0.0.1"},
		Timeout:      2 * time.Second,
		MaxBytes:     1024,
	})
	res, err := f.Fetch(tsvsheet.ImportURL(plain.URL), cellMedia)
	require.NoError(t, err)
	assert.Equal(t, "local", string(res.Body))
}

func TestNew_DefaultsClientAndInstallsCheckRedirect(t *testing.T) {
	t.Parallel()

	f := New(Config{})
	require.NotNil(t, f.client)               // nil Client → default built
	require.NotNil(t, f.client.CheckRedirect) // redirect guard installed
}

func TestNew_KeepsInjectedClient(t *testing.T) {
	t.Parallel()

	injected := &http.Client{}
	f := New(Config{Client: injected})
	assert.Same(t, injected, f.client)
	require.NotNil(t, injected.CheckRedirect) // installed onto the injected client
}

// TestContextFor_ZeroTimeoutIsNotAnExpiredDeadline pins the trap the doc names:
// context.WithTimeout(ctx, 0) yields an ALREADY-expired deadline, so a Fetcher
// built without an explicit timeout would fail every request instantly rather
// than running unbounded. The zero case must produce a plain cancelable
// context.
func TestContextFor_ZeroTimeoutIsNotAnExpiredDeadline(t *testing.T) {
	t.Parallel()

	ctx, cancel := New(Config{}).contextFor()
	defer cancel()

	_, hasDeadline := ctx.Deadline()
	assert.False(t, hasDeadline, "no configured timeout means no deadline at all")
	assert.NoError(t, ctx.Err(), "the context is live, not already expired")

	timed, cancelTimed := New(Config{Timeout: time.Minute}).contextFor()
	defer cancelTimed()
	_, hasDeadline = timed.Deadline()
	assert.True(t, hasDeadline, "a positive timeout does set one")
}

// TestConfig_NilClientGetsADefaultAndAlwaysOursCheckRedirect pins the injection
// contract: a caller may omit the client entirely, and either way the Fetcher
// installs its own CheckRedirect — an injected client that kept its own would
// follow redirects without re-validating the host, which is the whole point of
// the allowlist.
func TestConfig_NilClientGetsADefaultAndAlwaysOursCheckRedirect(t *testing.T) {
	t.Parallel()

	fromNil := New(Config{})
	require.NotNil(t, fromNil.client)
	assert.NotNil(t, fromNil.client.CheckRedirect)

	injected := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return nil }}
	fromInjected := New(Config{Client: injected})
	assert.Same(t, injected, fromInjected.client, "the injected client is kept")
	assert.NotNil(t, injected.CheckRedirect, "but its CheckRedirect is replaced with ours")
}
