// Package importer is the security-hardened net/http sheet.Fetcher for
// content-typed imports (ADR 0006 §7). It is the network security boundary: a
// frontend injects a configured Fetcher into the engine, and every IMPORT*
// fetch is funneled through it. The engine holds only the sheet.Fetcher
// interface; the allowlist, timeout, size cap, and redirect re-validation live
// here so the engine stays transport-free.
//
// Every failure is a distinct constants.ErrImport* sentinel (matchable with
// errors.Is) so callers and logs can tell a denied host from a bad status from
// an oversized body — the engine deliberately collapses them all to #IMPORT!.
package importer

import (
	"context"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/uplang/tsvsheet.go/internal/constants"
	"github.com/uplang/tsvsheet.go/internal/sheet"
)

// HostPattern is one allowlist entry: an exact host ("example.com") or a
// leading-"*." wildcard ("*.example.com") that matches any proper subdomain but
// never the apex.
type HostPattern string

// Host is a request URL's hostname (port and IPv6 brackets already stripped),
// checked against the allowlist.
type Host string

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
	AllowedHosts []HostPattern
	Timeout      time.Duration
	MaxBytes     ByteSize
}

// Fetcher is the concrete sheet.Fetcher. Its methods take value receivers and
// New returns it by value: the struct is effectively immutable after
// construction (the embedded *http.Client is the only reference type, and it is
// never reassigned), so no pointer is required.
type Fetcher struct {
	client   *http.Client
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
func (f Fetcher) Fetch(url sheet.ImportURL, accept sheet.MediaType) (sheet.FetchResult, error) {
	ctx, cancel := f.contextFor()
	defer cancel()
	req, err := f.request(ctx, url, accept)
	if err != nil {
		return sheet.FetchResult{}, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		closeBody(resp)
		return sheet.FetchResult{}, constants.ErrImportFetch.With(err)
	}
	defer func() { _ = resp.Body.Close() }()
	return f.result(resp)
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

// request builds the validated GET request: the URL must parse, its scheme must
// be exactly https, and its host must be allowlisted — otherwise the matching
// sentinel, before any network I/O.
func (f Fetcher) request(ctx context.Context, url sheet.ImportURL, accept sheet.MediaType) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, string(url), nil)
	if err != nil {
		return nil, constants.ErrImportURL.With(err)
	}
	if req.URL.Scheme != "https" {
		return nil, constants.ErrImportScheme
	}
	if !f.hostAllowed(Host(req.URL.Hostname())) {
		return nil, constants.ErrImportHostDenied
	}
	req.Header.Set("Accept", string(accept))
	return req, nil
}

// result turns a received response into a FetchResult: only a 2xx status is
// accepted, the body is read under the size cap, and the Content-Type is
// normalized to its base media type (parameters stripped).
func (f Fetcher) result(resp *http.Response) (sheet.FetchResult, error) {
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return sheet.FetchResult{}, constants.ErrImportStatus
	}
	body, err := f.readCapped(resp.Body)
	if err != nil {
		return sheet.FetchResult{}, err
	}
	base, err := normalizeContentType(resp)
	if err != nil {
		return sheet.FetchResult{}, err
	}
	return sheet.FetchResult{ContentType: base, Body: body}, nil
}

// readCapped reads at most f.maxBytes bytes: it reads one byte past the cap and
// rejects the whole body (never a truncation) if that extra byte materializes.
func (f Fetcher) readCapped(body io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, int64(f.maxBytes)+1))
	if err != nil {
		return nil, constants.ErrImportRead.With(err)
	}
	if ByteSize(len(data)) > f.maxBytes {
		return nil, constants.ErrImportTooLarge
	}
	return data, nil
}

// checkRedirect re-validates every redirect hop: too many hops, a non-https
// target, or a target host outside the allowlist is refused (never followed) —
// all as ErrImportRedirect. via holds the requests already made.
func (f Fetcher) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return constants.ErrImportRedirect
	}
	if req.URL.Scheme != "https" {
		return constants.ErrImportRedirect
	}
	if !f.hostAllowed(Host(req.URL.Hostname())) {
		return constants.ErrImportRedirect
	}
	return nil
}

// hostAllowed reports whether host matches any allowlist entry; an empty
// allowlist denies everything.
func (f Fetcher) hostAllowed(host Host) bool {
	for _, pattern := range f.allowed {
		if matchHost(pattern, host) {
			return true
		}
	}
	return false
}

// matchHost reports whether host satisfies pattern, case-insensitively: a
// leading "*." is a subdomain wildcard, anything else is an exact host.
func matchHost(pattern HostPattern, host Host) bool {
	p := strings.ToLower(string(pattern))
	h := strings.ToLower(string(host))
	if suffix, ok := strings.CutPrefix(p, "*."); ok {
		return wildcardMatch(Host(suffix), Host(h))
	}
	return p == h
}

// wildcardMatch reports whether host is a proper subdomain of suffix: host must
// end with "."+suffix AND carry a non-empty label before it. This rejects the
// apex ("example.com" does not end with ".example.com"), the lookalike
// ("evilexample.com" — the char before "example.com" is a letter, not a dot),
// and the bare-suffix trick (".example.com" — the label before the dot is
// empty).
func wildcardMatch(suffix, host Host) bool {
	label, ok := strings.CutSuffix(string(host), "."+string(suffix))
	return ok && label != ""
}

// normalizeContentType parses the response Content-Type and returns its base
// media type with parameters stripped (so a correctly-typed response carrying a
// charset param still matches the handshake). A malformed header is
// ErrImportContentType.
func normalizeContentType(resp *http.Response) (sheet.MediaType, error) {
	base, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return "", constants.ErrImportContentType.With(err)
	}
	return sheet.MediaType(base), nil
}

// closeBody closes a response body when the response is present — the redirect
// refusal path returns a non-nil response whose body the caller still owns,
// while a transport error returns none.
func closeBody(resp *http.Response) {
	if resp != nil {
		_ = resp.Body.Close()
	}
}
