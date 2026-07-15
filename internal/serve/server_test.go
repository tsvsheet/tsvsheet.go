package serve_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/serve"
	"github.com/uplang/tsvsheet.go/internal/session"
	"github.com/uplang/tsvsheet.go/internal/sheet"
)

// sampleSheet is a small spreadsheet: three data columns and a D-column formula
// summing B and C per row.
var sampleSheet = []byte(
	"name\tb\tc\ttotal\n" +
		"Alice\t2\t3\t=B2+C2\n" +
		"Bob\t4\t5\t=B3+C3\n",
)

// testServer builds a server over a fresh session and records whether save ran.
func testServer(t *testing.T) (serve.Server, *bool) {
	t.Helper()
	sess, err := session.New(sampleSheet)
	require.NoError(t, err)
	saved := false
	return serve.NewServer(sess, func() error { saved = true; return nil }), &saved
}

// do issues a request against a server's handler and returns the recorder.
func do(t *testing.T, srv serve.Server, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func TestState(t *testing.T) {
	t.Parallel()

	srv, _ := testServer(t)
	rec := do(t, srv, http.MethodGet, "/api/state", "")
	require.Equal(t, http.StatusOK, rec.Code)

	var state session.State
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &state))
	assert.Equal(t, "5", state.Computed[1][3]) // D2 = B2+C2
	assert.False(t, state.IsDirty)
}

func TestSetCell_OK(t *testing.T) {
	t.Parallel()

	srv, _ := testServer(t)
	rec := do(t, srv, http.MethodPut, "/api/cell", `{"row":1,"col":1,"text":"10"}`)
	require.Equal(t, http.StatusOK, rec.Code)

	var state session.State
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &state))
	assert.Equal(t, "13", state.Computed[1][3]) // D2 = 10+3
	assert.True(t, state.IsDirty)
}

func TestSetCell_FormulaSyntaxError(t *testing.T) {
	t.Parallel()

	srv, _ := testServer(t)
	rec := do(t, srv, http.MethodPut, "/api/cell", `{"row":1,"col":3,"text":"=sum("}`)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Contains(t, rec.Body.String(), "error")
}

func TestSetCell_InvalidAddress(t *testing.T) {
	t.Parallel()

	srv, _ := testServer(t)
	rec := do(t, srv, http.MethodPut, "/api/cell", `{"row":-1,"col":0,"text":"x"}`)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestSetCell_BadBody(t *testing.T) {
	t.Parallel()

	srv, _ := testServer(t)
	rec := do(t, srv, http.MethodPut, "/api/cell", `not json`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSave_OK(t *testing.T) {
	t.Parallel()

	srv, saved := testServer(t)
	rec := do(t, srv, http.MethodPost, "/api/save", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, *saved)
	assert.Contains(t, rec.Body.String(), `"saved":true`)
}

func TestSave_Error(t *testing.T) {
	t.Parallel()

	sess, err := session.New(sampleSheet)
	require.NoError(t, err)
	srv := serve.NewServer(sess, func() error { return errors.New("disk full") })

	rec := do(t, srv, http.MethodPost, "/api/save", "")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "disk full")
}

func TestExplain_OK(t *testing.T) {
	t.Parallel()

	srv, _ := testServer(t)
	rec := do(t, srv, http.MethodGet, "/api/explain?cell=D2", "")
	require.Equal(t, http.StatusOK, rec.Code)

	var trace sheet.Trace
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &trace))
	assert.Equal(t, "5", trace.Value)
	assert.Equal(t, "B2 + C2", trace.Formula)
}

func TestExplain_BadCell(t *testing.T) {
	t.Parallel()

	srv, _ := testServer(t)
	rec := do(t, srv, http.MethodGet, "/api/explain?cell=bogus", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestExplain_OutOfGrid(t *testing.T) {
	t.Parallel()

	srv, _ := testServer(t)
	rec := do(t, srv, http.MethodGet, "/api/explain?cell=Z99", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestReferences_OK(t *testing.T) {
	t.Parallel()

	// D2 (=B2+C2) reads B2 and C2; D2 itself is read by nothing.
	srv, _ := testServer(t)
	rec := do(t, srv, http.MethodGet, "/api/references?cell=D2", "")
	require.Equal(t, http.StatusOK, rec.Code)

	var refs struct {
		Precedents []sheet.Span    `json:"precedents"`
		Dependents []sheet.Address `json:"dependents"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &refs))
	require.Len(t, refs.Precedents, 2)
	assert.Equal(t, sheet.Address{Row: 1, Col: 1}, refs.Precedents[0].From) // B2
	assert.Equal(t, sheet.Address{Row: 1, Col: 2}, refs.Precedents[1].From) // C2
	assert.Empty(t, refs.Dependents)
}

func TestReferences_BadCell(t *testing.T) {
	t.Parallel()

	srv, _ := testServer(t)
	rec := do(t, srv, http.MethodGet, "/api/references?cell=bogus", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUI_ServesHTML(t *testing.T) {
	t.Parallel()

	srv, _ := testServer(t)
	rec := do(t, srv, http.MethodGet, "/", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "<!doctype html>")
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
}
