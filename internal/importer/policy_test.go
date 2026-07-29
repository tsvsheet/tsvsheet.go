package importer

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

func TestMatchHost_Wildcard(t *testing.T) {
	t.Parallel()

	const pat HostPattern = "*.example.com"
	cases := map[Host]bool{
		"a.example.com":   true,  // proper subdomain
		"x.y.example.com": true,  // deep subdomain
		"A.Example.CoM":   true,  // case-insensitive
		"example.com":     false, // apex is NOT matched by *.
		"evilexample.com": false, // lookalike: char before "example.com" is a letter, not a dot
		".example.com":    false, // bare-suffix trick: empty label
		"example.org":     false, // different domain
	}
	for host, want := range cases {
		assert.Equal(t, want, matchHost(pat, host), string(host))
	}
}

func TestMatchHost_Exact(t *testing.T) {
	t.Parallel()

	const pat HostPattern = "Example.COM"
	assert.True(t, matchHost(pat, "example.com"))    // case-insensitive exact
	assert.False(t, matchHost(pat, "a.example.com")) // exact does not match subdomains
	assert.False(t, matchHost(pat, "example.org"))
}

func TestSchemeAllowed(t *testing.T) {
	t.Parallel()

	// https is allowed for any host; http only for a loopback target; any other
	// scheme is rejected regardless of host.
	assert.True(t, schemeAllowed("https", "example.com"))
	assert.True(t, schemeAllowed("https", "127.0.0.1"))
	assert.True(t, schemeAllowed("http", "127.0.0.1"))
	assert.True(t, schemeAllowed("http", "localhost"))
	assert.False(t, schemeAllowed("http", "example.com"))
	assert.False(t, schemeAllowed("ftp", "127.0.0.1"))
	assert.False(t, schemeAllowed("file", "localhost"))
}

func TestIsLoopback(t *testing.T) {
	t.Parallel()

	cases := map[Host]bool{
		"localhost":   true,  // the loopback name
		"LocalHost":   true,  // case-insensitive
		"127.0.0.1":   true,  // IPv4 loopback
		"127.0.0.5":   true,  // anywhere in 127.0.0.0/8
		"::1":         true,  // IPv6 loopback
		"example.com": false, // a name that is not localhost
		"8.8.8.8":     false, // a routable IP
		"":            false, // empty is not loopback
	}
	for host, want := range cases {
		assert.Equal(t, want, IsLoopback(host), string(host))
	}
}

func TestHostAllowed_EmptyDeniesAll(t *testing.T) {
	t.Parallel()

	f := Fetcher{}
	assert.False(t, f.hostAllowed("example.com"))
}

func TestHostAllowed_FirstOfSeveral(t *testing.T) {
	t.Parallel()

	f := Fetcher{allowed: []HostPattern{"a.com", "*.b.com", "c.com"}}
	assert.True(t, f.hostAllowed("x.b.com")) // matched by the wildcard entry
	assert.False(t, f.hostAllowed("d.com"))  // exhausts the list
}

// errReader always fails, exercising readCapped's io.ReadAll error branch.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

// TestHostPattern_WildcardNeverMatchesTheApex pins the sharp edge of the
// allowlist: "*.example.com" grants the subdomains and NOT example.com itself.
// An operator allowlisting a subdomain wildcard has not allowlisted the apex,
// and a matcher that conflated them would silently widen every grant.
func TestHostPattern_WildcardNeverMatchesTheApex(t *testing.T) {
	t.Parallel()

	const pattern HostPattern = "*.example.com"
	assert.True(t, matchHost(pattern, "api.example.com"))
	assert.False(t, matchHost(pattern, "example.com"), "the apex is not granted")
	assert.False(t, matchHost(pattern, "evilexample.com"), "nor a lookalike")
}

func TestFetch_RedirectToAllowedHTTPSFollowed(t *testing.T) {
	t.Parallel()

	f, srv := tlsFetcher(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/final" {
			w.Header().Set("Content-Type", string(cellMedia))
			_, _ = w.Write([]byte("done"))
			return
		}
		http.Redirect(w, r, "/final", http.StatusFound) // same (allowed) host, https
	})
	res, err := f.Fetch(tsvsheet.ImportURL(srv.URL+"/start"), cellMedia)
	require.NoError(t, err)
	assert.Equal(t, "done", string(res.Body))
}

func TestFetch_RedirectToDisallowedHostRefused(t *testing.T) {
	t.Parallel()

	f, srv := tlsFetcher(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://evil.invalid/x", http.StatusFound)
	})
	_, err := f.Fetch(tsvsheet.ImportURL(srv.URL), cellMedia)
	assert.ErrorIs(t, err, constants.ErrImportRedirect)
}

func TestFetch_RedirectToNonLoopbackHTTPRefused(t *testing.T) {
	t.Parallel()

	// The redirect downgrades to http on a NON-loopback host that is itself
	// allowlisted, so the refusal is on scheme (not host): plain http is only
	// permitted for a loopback target. The hop is never actually followed.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://example.com/x", http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	f := New(Config{
		Client:       srv.Client(),
		AllowedHosts: []HostPattern{"127.0.0.1", "example.com"},
		Timeout:      2 * time.Second,
		MaxBytes:     1024,
	})
	_, err := f.Fetch(tsvsheet.ImportURL(srv.URL), cellMedia)
	assert.ErrorIs(t, err, constants.ErrImportRedirect)
}

func TestFetch_RedirectToLoopbackHTTPFollowed(t *testing.T) {
	t.Parallel()

	// A plain-http loopback endpoint is a legitimate redirect target (reaching a
	// local service is a primary import use case): the https→http-loopback hop is
	// followed and its body returned.
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", string(cellMedia))
		_, _ = w.Write([]byte("done"))
	}))
	t.Cleanup(plain.Close)
	f, srv := tlsFetcher(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plain.URL+"/final", http.StatusFound) // https → http-loopback
	})
	res, err := f.Fetch(tsvsheet.ImportURL(srv.URL+"/start"), cellMedia)
	require.NoError(t, err)
	assert.Equal(t, "done", string(res.Body))
}

func TestFetch_TooManyRedirects(t *testing.T) {
	t.Parallel()

	f, srv := tlsFetcher(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/again", http.StatusFound) // loops on the allowed host
	})
	_, err := f.Fetch(tsvsheet.ImportURL(srv.URL+"/start"), cellMedia)
	assert.ErrorIs(t, err, constants.ErrImportRedirect)
}

// ---- cache --------------------------------------------------------------

// countingFetcher records how many times its Fetch is called and returns a
// configurable result/error.
type countingFetcher struct {
	err   error
	res   tsvsheet.FetchResult
	calls atomic.Int64
}

func (c *countingFetcher) Fetch(_ tsvsheet.ImportURL, _ tsvsheet.MediaType) (tsvsheet.FetchResult, error) {
	c.calls.Add(1)
	return c.res, c.err
}
