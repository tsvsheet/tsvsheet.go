// Package importer's response half: turning a received HTTP response into a
// FetchResult under the size cap, with its Content-Type normalized to the base
// media type the handshake matches against.
package importer

import (
	"io"
	"mime"
	"net/http"

	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

// result turns a received response into a FetchResult: only a 2xx status is
// accepted, the body is read under the size cap, and the Content-Type is
// normalized to its base media type (parameters stripped).
func (f Fetcher) result(resp *http.Response) (tsvsheet.FetchResult, error) {
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return tsvsheet.FetchResult{}, constants.ErrImportStatus
	}
	body, err := f.readCapped(resp.Body)
	if err != nil {
		return tsvsheet.FetchResult{}, err
	}
	base, err := normalizeContentType(resp)
	if err != nil {
		return tsvsheet.FetchResult{}, err
	}
	return tsvsheet.FetchResult{ContentType: base, Body: body}, nil
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

// normalizeContentType parses the response Content-Type and returns its base
// media type with parameters stripped (so a correctly-typed response carrying a
// charset param still matches the handshake). A malformed header is
// ErrImportContentType.
func normalizeContentType(resp *http.Response) (tsvsheet.MediaType, error) {
	base, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return "", constants.ErrImportContentType.With(err)
	}
	return tsvsheet.MediaType(base), nil
}

// closeBody closes a response body when the response is present — the redirect
// refusal path returns a non-nil response whose body the caller still owns,
// while a transport error returns none.
func closeBody(resp *http.Response) {
	if resp != nil {
		_ = resp.Body.Close()
	}
}
