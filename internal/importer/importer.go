// Package importer is the security-hardened net/http tsvsheet.Fetcher for
// content-typed imports (ADR 0006 §7). It is the network security boundary: a
// frontend injects a configured Fetcher into the engine, and every IMPORT*
// fetch is funneled through it. The engine holds only the tsvsheet.Fetcher
// interface; the allowlist, timeout, size cap, and redirect re-validation live
// here so the engine stays transport-free.
//
// Every failure is a distinct constants.ErrImport* sentinel (matchable with
// errors.Is) so callers and logs can tell a denied host from a bad status from
// an oversized body — the engine deliberately collapses them all to #IMPORT!.
package importer

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

// ByteSize is a byte count — the maximum import body the Fetcher will read.
type ByteSize int64

// maxRedirects caps the redirect hops a single Fetch will follow before
// refusing; every hop is re-validated regardless (ADR 0006 §7).
const maxRedirects = 5

// Config is the injected Fetcher configuration (dependency injection — no
// globals, no package state). A nil Client is replaced with a default one; the
// Fetcher always installs its own CheckRedirect so every redirect hop is
// re-validated against this same allowlist.
type Config struct {
	Client       *http.Client
	Base         DataBase
	AllowedHosts []HostPattern
	Timeout      time.Duration
	MaxBytes     ByteSize
}

// Fetcher is the concrete tsvsheet.Fetcher. Its methods take value receivers and
// New returns it by value: the struct is effectively immutable after
// construction (the embedded *http.Client is the only reference type, and it is
// never reassigned), so no pointer is required.
type Fetcher struct {
	client   *http.Client
	base     DataBase
	allowed  []HostPattern
	timeout  time.Duration
	maxBytes ByteSize
}

// New builds a Fetcher from cfg. A nil cfg.Client becomes a default
// &http.Client{} with NO client-level timeout — the per-request context
// deadline (cfg.Timeout) bounds the whole exchange instead. Either way the
// client's CheckRedirect is replaced so every redirect hop is re-validated.
func New(cfg Config) Fetcher {
	client := cfg.Client
	if client == nil {
		client = &http.Client{}
	}
	f := Fetcher{
		client:   client,
		base:     cfg.Base,
		allowed:  cfg.AllowedHosts,
		timeout:  cfg.Timeout,
		maxBytes: cfg.MaxBytes,
	}
	client.CheckRedirect = f.checkRedirect
	return f
}

// Fetch retrieves url, sending accept as the Accept header, and returns the body
// with its normalized (parameter-stripped) Content-Type. Every failure is a
// distinct constants.ErrImport* sentinel.
func (f Fetcher) Fetch(url tsvsheet.ImportURL, accept tsvsheet.MediaType) (tsvsheet.FetchResult, error) {
	ctx, cancel := f.contextFor()
	defer cancel()
	req, err := f.request(ctx, url, accept)
	if err != nil {
		return tsvsheet.FetchResult{}, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		closeBody(resp)
		return tsvsheet.FetchResult{}, constants.ErrImportFetch.With(err)
	}
	defer func() { _ = resp.Body.Close() }()
	res, err := f.result(resp)
	// The engine passed a source, not a URL: a relative one was resolved here,
	// against a base the engine never sees. Report where it actually went so
	// EXPLAIN can show it.
	res.URL = tsvsheet.ImportURL(req.URL.String())
	return res, err
}

// contextFor returns the per-request context: a deadline of f.timeout, or a
// plain cancelable context when no positive timeout is configured (a zero
// timeout must not produce an already-expired deadline).
func (f Fetcher) contextFor() (context.Context, context.CancelFunc) {
	if f.timeout <= 0 {
		return context.WithCancel(context.Background())
	}
	return context.WithTimeout(context.Background(), f.timeout)
}

// request builds the validated GET request: the reference resolves (absolute as
// written, or relative to the data base and confined to it), then the resolved
// URL is authorized — otherwise the matching sentinel, before any network I/O.
func (f Fetcher) request(
	ctx context.Context,
	ref tsvsheet.ImportURL,
	accept tsvsheet.MediaType,
) (*http.Request, error) {
	target, isFromBase, err := f.resolve(ref)
	if err != nil {
		return nil, err
	}
	if err := f.authorize(target, isFromBase); err != nil {
		return nil, err
	}
	req := newRequest(ctx, target)
	req.Header.Set("Accept", accept.Accept())
	return req, nil
}

// newRequest builds the GET for an already-parsed URL. http.NewRequest would
// re-parse a string this package has just parsed (directly, or via
// ResolveReference over parsed inputs); its parse error would then be a branch
// no input reaches, and an untestable branch is worse than an explicit
// construction.
func newRequest(ctx context.Context, target *url.URL) *http.Request {
	return (&http.Request{
		Method:     http.MethodGet,
		URL:        target,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Host:       target.Host,
	}).WithContext(ctx)
}

// resolve maps an import reference to the URL to fetch. A reference carrying a
// scheme is absolute and passes through unchanged. A reference with a host but
// no scheme ("//elsewhere/x") is refused: url.Parse reports it as relative, so
// resolving it against the base would silently retarget the host — the one way
// a sheet could otherwise choose its own server. Everything else is relative to
// the data base.
func (f Fetcher) resolve(ref tsvsheet.ImportURL) (*url.URL, baseOrigin, error) {
	parsed, err := url.Parse(string(ref))
	if err != nil {
		return nil, false, constants.ErrImportURL.With(err)
	}
	if parsed.IsAbs() {
		return parsed, false, nil
	}
	if parsed.Host != "" {
		return nil, false, constants.ErrImportURL
	}
	target, err := f.base.resolve(parsed)
	return target, true, err
}

// authorize applies the scheme and allowlist policy to a resolved URL. A
// reference that came from the data base is already authorized — the operator
// named that base, and its scheme was validated when it was built — so the
// allowlist, which governs URLs written inside a sheet, does not apply.
func (f Fetcher) authorize(target *url.URL, isFromBase baseOrigin) error {
	if isFromBase {
		return nil
	}
	if !schemeAllowed(urlScheme(target.Scheme), Host(target.Hostname())) {
		return constants.ErrImportScheme
	}
	if !f.hostAllowed(Host(target.Hostname())) {
		return constants.ErrImportHostDenied
	}
	return nil
}

// checkRedirect re-validates every redirect hop: too many hops, a scheme not
// permitted for the target host (http to a non-loopback hop), or a target host
// outside the allowlist is refused (never followed) — all as ErrImportRedirect.
// via holds the requests already made.
func (f Fetcher) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return constants.ErrImportRedirect
	}
	if !schemeAllowed(urlScheme(req.URL.Scheme), Host(req.URL.Hostname())) {
		return constants.ErrImportRedirect
	}
	if !f.hostAllowed(Host(req.URL.Hostname())) {
		return constants.ErrImportRedirect
	}
	return nil
}
