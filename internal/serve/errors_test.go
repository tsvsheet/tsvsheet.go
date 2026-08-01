package serve

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

// TestServerSentinels pins the errors this package emits, by identity rather
// than by status code: a caller (and a future refactor) must be able to tell a
// refused cross-origin write from a missing cell, and a status alone cannot.
func TestServerSentinels(t *testing.T) {
	t.Parallel()

	// A cross-origin write is refused with the sentinel in its body, so a
	// client reading the response learns which rule stopped it.
	guarded := guardCSRF(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/cell", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	guarded.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), constants.ErrForbidden.Error())
	assert.ErrorIs(t, constants.ErrForbidden.With(nil), constants.ErrForbidden)

	// A same-site write is refused too: a sub-domain a browser labels
	// "same-site" is still not this origin, and the guard's whole purpose is
	// that a page elsewhere cannot write here.
	sameSite := httptest.NewRecorder()
	siteReq := httptest.NewRequest(http.MethodPost, "/api/cell", nil)
	siteReq.Header.Set("Sec-Fetch-Site", "same-site")
	guarded.ServeHTTP(sameSite, siteReq)
	assert.Equal(t, http.StatusForbidden, sameSite.Code)
	assert.Contains(t, sameSite.Body.String(), constants.ErrForbidden.Error())

	// A same-origin write passes through untouched.
	passed := httptest.NewRecorder()
	same := httptest.NewRequest(http.MethodPost, "/api/cell", nil)
	same.Header.Set("Sec-Fetch-Site", "same-origin")
	guarded.ServeHTTP(passed, same)
	assert.Equal(t, http.StatusOK, passed.Code)
}

// TestNotFoundSentinel pins the error a request for a cell outside the grid
// produces — by driving the handler, not by asserting that a sentinel is
// itself. An earlier version of this test did the latter and could not tell
// this error apart from any other.
func TestNotFoundSentinel(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	testServerHandler(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/embedded?cell=ZZ99", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), tsvsheet.ErrNotFound.Error())
	assert.NotContains(t, rec.Body.String(), tsvsheet.ErrInvalidValue.Error(),
		"the response names THIS refusal, not a neighbouring one")

	// And by identity, so the sentinel cannot be swapped for a neighbour
	// while the status stays 404.
	srv, _ := testServer(t)
	_, err := srv.embeddedAt(tsvsheet.Address{Row: 98, Col: 700})
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrNotFound)
	assert.NotErrorIs(t, err, tsvsheet.ErrInvalidValue)
}

// testServerHandler is a server over the sample sheet, as a handler.
func testServerHandler(t *testing.T) http.Handler {
	t.Helper()
	srv, _ := testServer(t)
	return srv.Handler()
}
