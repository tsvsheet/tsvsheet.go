package importer_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
	"github.com/tsvsheet/tsvsheet.go/internal/importer"
)

const cellMedia = tsvsheet.MediaType("application/vnd.tsvsheet.cell+tsv")

// tlsFetcher stands up a TLS test server with handler h and returns a Fetcher
// wired to trust it, allowlisting the server's 127.0.0.1 host.
func tlsFetcher(t *testing.T, h http.HandlerFunc) (importer.Fetcher, *httptest.Server) {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	cfg := importer.Config{
		Client:       srv.Client(),
		AllowedHosts: []importer.HostPattern{"127.0.0.1"},
		Timeout:      2 * time.Second,
		MaxBytes:     1024,
	}
	return importer.New(cfg), srv
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

	f := importer.New(importer.Config{AllowedHosts: []importer.HostPattern{"example.com"}})
	for _, raw := range []string{"http://example.com/x", "file:///etc/passwd", "ftp://example.com/x"} {
		_, err := f.Fetch(tsvsheet.ImportURL(raw), cellMedia)
		assert.ErrorIs(t, err, constants.ErrImportScheme, raw)
	}
}

func TestFetch_MalformedURL(t *testing.T) {
	t.Parallel()

	f := importer.New(importer.Config{AllowedHosts: []importer.HostPattern{"example.com"}})
	// A DEL control character makes net/url (and thus NewRequestWithContext) reject the URL.
	_, err := f.Fetch(tsvsheet.ImportURL("https://example.com/\x7f"), cellMedia)
	assert.ErrorIs(t, err, constants.ErrImportURL)
}

func TestFetch_HostDenied_NilClientDefault(t *testing.T) {
	t.Parallel()

	// Empty allowlist denies everything; a nil Client exercises New's default-client branch.
	f := importer.New(importer.Config{})
	_, err := f.Fetch(tsvsheet.ImportURL("https://example.com/x"), cellMedia)
	assert.ErrorIs(t, err, constants.ErrImportHostDenied)
}

func TestFetch_NonAllowlistedHostDenied(t *testing.T) {
	t.Parallel()

	f := importer.New(importer.Config{AllowedHosts: []importer.HostPattern{"good.example.com"}})
	_, err := f.Fetch(tsvsheet.ImportURL("https://evil.example.com/x"), cellMedia)
	assert.ErrorIs(t, err, constants.ErrImportHostDenied)
}

func TestFetch_Non2xxStatus(t *testing.T) {
	t.Parallel()

	f, srv := tlsFetcher(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	})
	_, err := f.Fetch(tsvsheet.ImportURL(srv.URL), cellMedia)
	assert.ErrorIs(t, err, constants.ErrImportStatus)
}

func TestFetch_MalformedContentType(t *testing.T) {
	t.Parallel()

	f, srv := tlsFetcher(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/foo; bar") // param without value → parse error
		_, _ = w.Write([]byte("x"))
	})
	_, err := f.Fetch(tsvsheet.ImportURL(srv.URL), cellMedia)
	assert.ErrorIs(t, err, constants.ErrImportContentType)
}

func TestFetch_BodyAtLimitOK(t *testing.T) {
	t.Parallel()

	body := make([]byte, 1024) // exactly MaxBytes
	for i := range body {
		body[i] = 'a'
	}
	f, srv := tlsFetcher(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", string(cellMedia))
		_, _ = w.Write(body)
	})
	res, err := f.Fetch(tsvsheet.ImportURL(srv.URL), cellMedia)
	require.NoError(t, err)
	assert.Len(t, res.Body, 1024)
}

func TestFetch_BodyOverLimitRejected(t *testing.T) {
	t.Parallel()

	body := make([]byte, 1025) // one past MaxBytes
	f, srv := tlsFetcher(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", string(cellMedia))
		_, _ = w.Write(body)
	})
	_, err := f.Fetch(tsvsheet.ImportURL(srv.URL), cellMedia)
	assert.ErrorIs(t, err, constants.ErrImportTooLarge)
}

func TestFetch_Timeout(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond) // outlast the 20ms client deadline, then finish
		w.Header().Set("Content-Type", string(cellMedia))
	}))
	t.Cleanup(srv.Close)
	f := importer.New(importer.Config{
		AllowedHosts: []importer.HostPattern{"127.0.0.1"},
		Timeout:      20 * time.Millisecond,
		MaxBytes:     1024,
		Client:       srv.Client(),
	})

	_, err := f.Fetch(tsvsheet.ImportURL(srv.URL), cellMedia)
	assert.ErrorIs(t, err, constants.ErrImportFetch)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// ---- redirects ----------------------------------------------------------

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
	f := importer.New(importer.Config{
		Client:       srv.Client(),
		AllowedHosts: []importer.HostPattern{"127.0.0.1", "example.com"},
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

func TestFetch_LoopbackHTTPAllowed(t *testing.T) {
	t.Parallel()

	// A direct plain-http request to a loopback host is permitted: http is
	// allowed when the target is loopback.
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", string(cellMedia))
		_, _ = w.Write([]byte("local"))
	}))
	t.Cleanup(plain.Close)
	f := importer.New(importer.Config{
		AllowedHosts: []importer.HostPattern{"127.0.0.1"},
		Timeout:      2 * time.Second,
		MaxBytes:     1024,
	})
	res, err := f.Fetch(tsvsheet.ImportURL(plain.URL), cellMedia)
	require.NoError(t, err)
	assert.Equal(t, "local", string(res.Body))
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

func TestCache_MissThenHitNoSecondFetch(t *testing.T) {
	t.Parallel()

	inner := &countingFetcher{res: tsvsheet.FetchResult{ContentType: cellMedia, Body: []byte("v")}}
	c := importer.NewCache(inner)

	first, err := c.Fetch("https://x/a", cellMedia)
	require.NoError(t, err)
	assert.Equal(t, "v", string(first.Body))

	second, err := c.Fetch("https://x/a", cellMedia)
	require.NoError(t, err)
	assert.Equal(t, "v", string(second.Body))
	assert.Equal(t, int64(1), inner.calls.Load()) // second served from cache
}

func TestCache_ClearDropsEntries(t *testing.T) {
	t.Parallel()

	inner := &countingFetcher{res: tsvsheet.FetchResult{ContentType: cellMedia, Body: []byte("v")}}
	c := importer.NewCache(inner)

	_, _ = c.Fetch("https://x/a", cellMedia)
	c.Clear()
	_, _ = c.Fetch("https://x/a", cellMedia)
	assert.Equal(t, int64(2), inner.calls.Load()) // refetched after Clear
}

func TestCache_KeyedByURLAndAccept(t *testing.T) {
	t.Parallel()

	inner := &countingFetcher{res: tsvsheet.FetchResult{ContentType: cellMedia, Body: []byte("v")}}
	c := importer.NewCache(inner)

	_, _ = c.Fetch("https://x/a", cellMedia)
	_, _ = c.Fetch("https://x/b", cellMedia)               // different url
	_, _ = c.Fetch("https://x/a", "application/other+tsv") // different accept
	assert.Equal(t, int64(3), inner.calls.Load())
}

func TestCache_ErrorNotCached(t *testing.T) {
	t.Parallel()

	inner := &countingFetcher{err: constants.ErrImportFetch}
	c := importer.NewCache(inner)

	_, err := c.Fetch("https://x/a", cellMedia)
	assert.ErrorIs(t, err, constants.ErrImportFetch)
	_, err = c.Fetch("https://x/a", cellMedia)
	assert.ErrorIs(t, err, constants.ErrImportFetch)
	assert.Equal(t, int64(2), inner.calls.Load()) // retried, not cached
}

func TestCache_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	inner := &countingFetcher{res: tsvsheet.FetchResult{ContentType: cellMedia, Body: []byte("v")}}
	c := importer.NewCache(inner)

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			url := tsvsheet.ImportURL("https://x/a")
			if n%2 == 0 {
				c.Clear()
			}
			_, _ = c.Fetch(url, cellMedia)
		}(i)
	}
	wg.Wait()
}
