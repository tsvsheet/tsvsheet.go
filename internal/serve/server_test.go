package serve_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/refresh"
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
	return serve.NewServer(sess, func() error { saved = true; return nil }, nil), &saved
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
	srv := serve.NewServer(sess, func() error { return errors.New("disk full") }, nil)

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

func TestStructure_AllOps(t *testing.T) {
	t.Parallel()

	// sampleSheet is 3 rows × 4 columns; each op reshapes it relative to (1,1).
	cases := []struct {
		op       string
		wantRows int
		wantCols int
	}{
		{"insert-row", 4, 4},
		{"delete-row", 2, 4},
		{"insert-col", 3, 5},
		{"delete-col", 3, 3},
	}
	for _, tc := range cases {
		t.Run(tc.op, func(t *testing.T) {
			t.Parallel()
			srv, _ := testServer(t)
			rec := do(t, srv, http.MethodPost, "/api/structure", fmt.Sprintf(`{"op":%q,"row":1,"col":1}`, tc.op))
			require.Equal(t, http.StatusOK, rec.Code)

			var state session.State
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &state))
			assert.Len(t, state.Source, tc.wantRows)
			assert.Len(t, state.Source[0], tc.wantCols)
			assert.True(t, state.IsDirty)
		})
	}
}

func TestStructure_UnknownOp(t *testing.T) {
	t.Parallel()

	srv, _ := testServer(t)
	rec := do(t, srv, http.MethodPost, "/api/structure", `{"op":"bogus","row":0,"col":0}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestStructure_BadBody(t *testing.T) {
	t.Parallel()

	srv, _ := testServer(t)
	rec := do(t, srv, http.MethodPost, "/api/structure", `not json`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestEmbedded_OK(t *testing.T) {
	t.Parallel()

	loader := func(_, ref sheet.Path) (sheet.Sheet, sheet.Path, error) {
		s, err := sheet.Parse([]byte("=output(9)\n"))
		return s, ref, err
	}
	sess, err := session.NewEmbeddable([]byte("=sheet(\"c\")\n"), loader, "root", sheet.DefaultLimits())
	require.NoError(t, err)
	srv := serve.NewServer(sess, func() error { return nil }, nil)

	rec := do(t, srv, http.MethodGet, "/api/embedded?cell=A1", "")
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Path string     `json:"path"`
		Grid [][]string `json:"grid"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "c", resp.Path)
	assert.Equal(t, "9", resp.Grid[0][0])
}

func TestEmbedded_NotAnEmbedIs404(t *testing.T) {
	t.Parallel()

	// D2 in sampleSheet is a formula, but not a SHEET call.
	srv, _ := testServer(t)
	rec := do(t, srv, http.MethodGet, "/api/embedded?cell=D2", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestEmbedded_BadCell(t *testing.T) {
	t.Parallel()

	srv, _ := testServer(t)
	rec := do(t, srv, http.MethodGet, "/api/embedded?cell=bogus", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestConfig_RefreshMillis(t *testing.T) {
	t.Parallel()

	sess, err := session.New([]byte("=now()\n"))
	require.NoError(t, err)
	srv := serve.NewServer(sess, func() error { return nil }, refresh.Every(2*time.Second))

	rec := do(t, srv, http.MethodGet, "/api/config", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"next_refresh_millis":2000`)
}

func TestConfig_NoRefresh(t *testing.T) {
	t.Parallel()

	// A nil cadence and a zero cadence both report no next refresh.
	srv, _ := testServer(t) // nil refresh
	assert.Contains(t, do(t, srv, http.MethodGet, "/api/config", "").Body.String(), `"next_refresh_millis":0`)

	sess, err := session.New(sampleSheet)
	require.NoError(t, err)
	zero := serve.NewServer(sess, func() error { return nil }, refresh.Every(0))
	assert.Contains(t, do(t, zero, http.MethodGet, "/api/config", "").Body.String(), `"next_refresh_millis":0`)
}

func TestRecompute_ReturnsFreshState(t *testing.T) {
	t.Parallel()

	srv, _ := testServer(t)
	rec := do(t, srv, http.MethodPost, "/api/recompute", "")
	require.Equal(t, http.StatusOK, rec.Code)

	var state session.State
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &state))
	assert.Equal(t, "5", state.Computed[1][3]) // D2 recomputed
}

func TestCSRF_CrossSiteMutationRefused(t *testing.T) {
	t.Parallel()

	// A cross-site state-changing request is refused (403); a safe GET is not.
	srv, _ := testServer(t)

	save := httptest.NewRequest(http.MethodPost, "/api/save", nil)
	save.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, save)
	assert.Equal(t, http.StatusForbidden, rec.Code)

	read := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	read.Header.Set("Sec-Fetch-Site", "cross-site") // safe method → allowed
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, read)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCSRF_SameOriginMutationAllowed(t *testing.T) {
	t.Parallel()

	// A same-origin mutation passes the guard.
	srv, _ := testServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/save", nil)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUI_ServesHTML(t *testing.T) {
	t.Parallel()

	srv, _ := testServer(t)
	rec := do(t, srv, http.MethodGet, "/", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "<!doctype html>")
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
}
