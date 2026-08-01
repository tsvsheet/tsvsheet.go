package serve

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/session"
)

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
		{"duplicate-row", 4, 4},
		{"duplicate-col", 3, 5},
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

func TestStructure_NegativeIndexRejected(t *testing.T) {
	t.Parallel()

	// A negative index must be a clean 400 at the boundary, never an engine
	// slice-bounds panic (insert-row with row:-1 crashes the released engine).
	srv, _ := testServer(t)
	for _, body := range []string{
		`{"op":"insert-row","row":-1,"col":0}`,
		`{"op":"insert-col","row":0,"col":-1}`,
	} {
		rec := do(t, srv, http.MethodPost, "/api/structure", body)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	}
}

func TestStructure_BadBody(t *testing.T) {
	t.Parallel()

	srv, _ := testServer(t)
	rec := do(t, srv, http.MethodPost, "/api/structure", `not json`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestStructure_FillOps(t *testing.T) {
	t.Parallel()

	// fill-down copies the cell above the selection into it, rebased —
	// Excel's single-cell Ctrl+D; fill-right the cell to its left.
	srv, _ := testServer(t)
	rec := do(t, srv, http.MethodPost, "/api/structure", `{"op":"fill-down","row":2,"col":3}`)
	require.Equal(t, http.StatusOK, rec.Code)
	var state session.State
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &state))
	assert.Equal(t, "=B3 + C3", state.Source[2][3])

	rec = do(t, srv, http.MethodPost, "/api/structure", `{"op":"fill-right","row":0,"col":1}`)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &state))
	assert.Equal(t, state.Source[0][0], state.Source[0][1]) // header copied left→right
}

func TestStructure_FillWithoutNeighborIsNoOp(t *testing.T) {
	t.Parallel()

	// The top row has no cell above, the first column none to the left: both
	// fills are quiet no-ops, as in Excel.
	srv, _ := testServer(t)
	for _, body := range []string{
		`{"op":"fill-down","row":0,"col":1}`,
		`{"op":"fill-right","row":1,"col":0}`,
	} {
		rec := do(t, srv, http.MethodPost, "/api/structure", body)
		require.Equal(t, http.StatusOK, rec.Code)
		var state session.State
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &state))
		assert.False(t, state.IsDirty) // the session was never touched
	}
}

// TestStructureSentinel pins the error a malformed structural request
// produces — by driving the handler. Asserting that a sentinel is itself, as
// an earlier version did, would pass even if this endpoint returned a
// completely different error.
func TestStructureSentinel(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"op":"nope","index":0}`)
	req := httptest.NewRequest(http.MethodPost, "/api/structure", body)
	testServerHandler(t).ServeHTTP(rec, req)
	assert.Contains(t, rec.Body.String(), tsvsheet.ErrInvalidValue.Error())
	assert.NotContains(t, rec.Body.String(), tsvsheet.ErrNotFound.Error())

	srv, _ := testServer(t)
	err := srv.structuralEdit(structureRequest{Op: "nope"})
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrInvalidValue)
	assert.NotErrorIs(t, err, tsvsheet.ErrNotFound)
}
