package serve_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/serve"
	"github.com/tsvsheet/tsvsheet.go/internal/session"
)

func TestReferences_OK(t *testing.T) {
	t.Parallel()

	// D2 (=B2+C2) reads B2 and C2; D2 itself is read by nothing.
	srv, _ := testServer(t)
	rec := do(t, srv, http.MethodGet, "/api/references?cell=D2", "")
	require.Equal(t, http.StatusOK, rec.Code)

	var refs struct {
		Precedents []tsvsheet.Span    `json:"precedents"`
		Dependents []tsvsheet.Address `json:"dependents"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &refs))
	require.Len(t, refs.Precedents, 2)
	assert.Equal(t, tsvsheet.Address{Row: 1, Col: 1}, refs.Precedents[0].From) // B2
	assert.Equal(t, tsvsheet.Address{Row: 1, Col: 2}, refs.Precedents[1].From) // C2
	assert.Empty(t, refs.Dependents)
}

func TestReferences_BadCell(t *testing.T) {
	t.Parallel()

	srv, _ := testServer(t)
	rec := do(t, srv, http.MethodGet, "/api/references?cell=bogus", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestEmbedded_OK(t *testing.T) {
	t.Parallel()

	loader := func(_, ref tsvsheet.Path) (tsvsheet.Sheet, tsvsheet.Path, error) {
		s, err := tsvsheet.Parse([]byte("=output(9)\n"))
		return s, ref, err
	}
	sess, err := session.NewEmbeddable([]byte("=sheet(\"c\")\n"), loader, "root", tsvsheet.DefaultLimits(), nil)
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
