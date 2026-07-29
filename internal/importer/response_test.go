package importer

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

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

func TestReadCapped_ReadError(t *testing.T) {
	t.Parallel()

	f := Fetcher{maxBytes: 16}
	_, err := f.readCapped(errReader{})
	assert.ErrorIs(t, err, constants.ErrImportRead)
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
}
