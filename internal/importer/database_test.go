package importer

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

func TestNewDataBase_SchemePolicyHasNoExemption(t *testing.T) {
	t.Parallel()

	cases := map[string]error{
		"https://data.example.com/team/": nil,                       // https anywhere
		"http://127.0.0.1:8137/":         nil,                       // http to loopback
		"http://localhost:8137/":         nil,                       // loopback by name
		"http://data.example.com/team/":  constants.ErrImportScheme, // cleartext to a remote host
		"ftp://data.example.com/":        constants.ErrImportScheme, // not http(s)
		"data/balances.tsv":              constants.ErrImportScheme, // no scheme at all
	}
	for raw, want := range cases {
		_, err := NewDataBase(BaseURL(raw))
		if want == nil {
			require.NoError(t, err, raw)
			continue
		}
		require.ErrorIs(t, err, want, raw)
	}
}

func TestNewDataBase_MalformedURL(t *testing.T) {
	t.Parallel()

	_, err := NewDataBase(BaseURL("https://exa mple.com/\x7f"))
	require.ErrorIs(t, err, constants.ErrImportURL)
}

func TestBasePrefix_NormalizesToTrailingSlash(t *testing.T) {
	t.Parallel()

	cases := map[basePath]basePath{
		"":           "/",
		"/":          "/",
		"/team":      "/team/",
		"/team/":     "/team/",
		"/a/../team": "/team/",
		"/team/sub/": "/team/sub/",
	}
	for in, want := range cases {
		assert.Equal(t, want, basePrefix(in), string(in))
	}
}

func TestFetcherResolve_AbsolutePassesThroughUnchanged(t *testing.T) {
	t.Parallel()

	f := New(Config{})
	got, fromBase, err := f.resolve("https://example.org/series.tsv")
	require.NoError(t, err)
	assert.Equal(t, "https://example.org/series.tsv", got.String())
	assert.False(t, bool(fromBase), "an absolute URL is not a base reference and stays under the allowlist")
}

func TestFetcherResolve_RelativeResolvesUnderTheBase(t *testing.T) {
	t.Parallel()

	base, err := NewDataBase(BaseURL("https://data.example.com/team"))
	require.NoError(t, err)
	f := New(Config{Base: base})

	cases := map[tsvsheet.ImportURL]string{
		"balances.tsv":   "https://data.example.com/team/balances.tsv",
		"macro/nfci.tsv": "https://data.example.com/team/macro/nfci.tsv",
		"./balances.tsv": "https://data.example.com/team/balances.tsv",
		"sub/../x.tsv":   "https://data.example.com/team/x.tsv",
	}
	for ref, want := range cases {
		got, fromBase, err := f.resolve(ref)
		require.NoError(t, err, string(ref))
		assert.Equal(t, want, got.String(), string(ref))
		assert.True(t, bool(fromBase), string(ref))
	}
}

func TestFetcherResolve_TraversalAboveTheBaseIsRefused(t *testing.T) {
	t.Parallel()

	base, err := NewDataBase(BaseURL("https://data.example.com/team/"))
	require.NoError(t, err)
	f := New(Config{Base: base})

	// Every shape of climb-out, including the escaped form and the sibling whose
	// name merely starts the same way.
	for _, ref := range []tsvsheet.ImportURL{
		"../../admin/keys.tsv",
		"../admin/keys.tsv",
		"sub/../../../etc/passwd",
		"%2e%2e/admin/keys.tsv",
		"/etc/passwd",
	} {
		_, _, err := f.resolve(ref)
		require.ErrorIs(t, err, constants.ErrImportEscape, string(ref))
	}
}

func TestFetcherResolve_SiblingPrefixIsNotUnderTheBase(t *testing.T) {
	t.Parallel()

	base, err := NewDataBase(BaseURL("https://data.example.com/team/"))
	require.NoError(t, err)
	f := New(Config{Base: base})

	_, _, err = f.resolve("../teamster/secrets.tsv")
	require.ErrorIs(t, err, constants.ErrImportEscape)
}

func TestFetcherResolve_SchemeRelativeReferenceCannotRetargetTheHost(t *testing.T) {
	t.Parallel()

	base, err := NewDataBase(BaseURL("https://data.example.com/team/"))
	require.NoError(t, err)
	f := New(Config{Base: base})

	// url.Parse reports "//elsewhere/x" as relative (no scheme), so resolving it
	// would point at elsewhere.example — the one way a sheet could choose its
	// own server. It must be refused, not resolved.
	_, _, err = f.resolve("//elsewhere.example/x.tsv")
	require.ErrorIs(t, err, constants.ErrImportURL)
}

func TestFetcherResolve_RelativeWithNoBaseIsRefused(t *testing.T) {
	t.Parallel()

	f := New(Config{})
	_, _, err := f.resolve("balances.tsv")
	require.ErrorIs(t, err, constants.ErrImportNoBase)
}

func TestFetcherResolve_MalformedReference(t *testing.T) {
	t.Parallel()

	f := New(Config{})
	_, _, err := f.resolve("://nonsense")
	require.ErrorIs(t, err, constants.ErrImportURL)
}

// tabularServer answers every request with a TSV body, and records the paths it
// was asked for so a test can assert what the base resolved to.
func tabularServer(t *testing.T, seen *[]string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = append(*seen, r.URL.Path)
		w.Header().Set("Content-Type", "text/tab-separated-values")
		_, _ = w.Write([]byte("Brokerage\t310000\n"))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestFetch_RelativeReferenceResolvesAgainstTheBaseWithNoAllowlist(t *testing.T) {
	t.Parallel()

	var seen []string
	server := tabularServer(t, &seen)
	base, err := NewDataBase(BaseURL(server.URL + "/team"))
	require.NoError(t, err)

	// No AllowedHosts at all: naming the base is the authorization, and it must
	// not require handing out network permission via --import-host.
	fetcher := New(Config{Base: base, MaxBytes: 1 << 20})

	res, err := fetcher.Fetch("balances.tsv", "application/vnd.tsvsheet+tsv")
	require.NoError(t, err)
	assert.Equal(t, "Brokerage\t310000\n", string(res.Body))
	assert.Equal(t, []string{"/team/balances.tsv"}, seen, "resolved under the base, not beside it")
}

func TestFetch_AbsoluteURLStillNeedsTheAllowlistEvenWithABase(t *testing.T) {
	t.Parallel()

	var seen []string
	server := tabularServer(t, &seen)
	base, err := NewDataBase(BaseURL(server.URL + "/team/"))
	require.NoError(t, err)
	fetcher := New(Config{Base: base, MaxBytes: 1 << 20})

	// The very same host, written absolutely in the sheet, is refused: a base
	// authorizes the base, never a host.
	_, err = fetcher.Fetch(tsvsheet.ImportURL(server.URL+"/team/balances.tsv"), "application/vnd.tsvsheet+tsv")
	require.ErrorIs(t, err, constants.ErrImportHostDenied)
	assert.Empty(t, seen, "refused before any network I/O")
}

func TestFetch_RelativeReferenceWithNoBaseNeverReachesTheNetwork(t *testing.T) {
	t.Parallel()

	var seen []string
	_ = tabularServer(t, &seen)
	fetcher := New(Config{MaxBytes: 1 << 20})

	_, err := fetcher.Fetch("balances.tsv", "application/vnd.tsvsheet+tsv")
	require.ErrorIs(t, err, constants.ErrImportNoBase)
	assert.Empty(t, seen)
}

func TestFetch_TraversalAboveTheBaseNeverReachesTheNetwork(t *testing.T) {
	t.Parallel()

	var seen []string
	server := tabularServer(t, &seen)
	base, err := NewDataBase(BaseURL(server.URL + "/team/"))
	require.NoError(t, err)
	fetcher := New(Config{Base: base, MaxBytes: 1 << 20})

	_, err = fetcher.Fetch("../../admin/keys.tsv", "application/vnd.tsvsheet+tsv")
	require.ErrorIs(t, err, constants.ErrImportEscape)
	assert.Empty(t, seen, "the refusal happens before the request is built")
}

func TestFetch_UnparseableReferenceIsRejected(t *testing.T) {
	t.Parallel()

	base, err := NewDataBase(BaseURL("http://127.0.0.1:1/"))
	require.NoError(t, err)
	fetcher := New(Config{Base: base, MaxBytes: 1 << 20})

	// Resolves against the base, but is not a URL http.NewRequest will accept.
	_, err = fetcher.Fetch("\x7f\x00", "application/vnd.tsvsheet+tsv")
	require.ErrorIs(t, err, constants.ErrImportURL)
}

func TestLoopbackBase_ResolvesRelativeReferences(t *testing.T) {
	t.Parallel()

	var seen []string
	server := tabularServer(t, &seen)
	// The shape startScopedData uses: a base built from an address this process
	// just bound, never parsed from text.
	fetcher := New(Config{
		Base:     LoopbackBase(HostPort(server.Listener.Addr().String())),
		MaxBytes: 1 << 20,
	})

	res, err := fetcher.Fetch("balances.tsv", "application/vnd.tsvsheet+tsv")
	require.NoError(t, err)
	assert.Equal(t, "Brokerage\t310000\n", string(res.Body))
	assert.Equal(t, []string{"/balances.tsv"}, seen)
}
