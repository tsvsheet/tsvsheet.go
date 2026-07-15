package sheet_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/sheet"
)

// addr is a test helper: a 0-based cell address.
func addr(row, col int) sheet.Address { return sheet.Address{Row: row, Col: col} }

func TestPrecedents_SingleAndRange(t *testing.T) {
	t.Parallel()

	// C1 = A1 + sum(A1:B2): a single-cell ref and a two-cell range.
	s := parse(t, "1\t2\t=A1 + sum(A1:B2)\n")
	spans := s.Precedents(addr(0, 2))
	require.Len(t, spans, 2)
	assert.Equal(t, sheet.Span{From: addr(0, 0), To: addr(0, 0)}, spans[0]) // A1
	assert.Equal(t, sheet.Span{From: addr(0, 0), To: addr(1, 1)}, spans[1]) // A1:B2
}

func TestPrecedents_LiteralOrOffGridOrNoRefs(t *testing.T) {
	t.Parallel()

	s := parse(t, "5\t=1 + 2\n")
	assert.Nil(t, s.Precedents(addr(0, 0))) // literal cell
	assert.Nil(t, s.Precedents(addr(0, 1))) // formula with no references
	assert.Nil(t, s.Precedents(addr(9, 9))) // off the grid
}

func TestPrecedents_MalformedRowZeroReferenceSkipped(t *testing.T) {
	t.Parallel()

	// The grammar admits A0 / A1:B0, but a flat grid has no row 0 — refSpan
	// drops such references (both the From and the To endpoint paths).
	assert.Nil(t, parse(t, "=A0\n").Precedents(addr(0, 0)))
	assert.Nil(t, parse(t, "=A1:B0\n").Precedents(addr(0, 0)))
}

func TestDependents_ReverseEdge(t *testing.T) {
	t.Parallel()

	// A1 is read by B1 (=A1) and C1 (=sum(A1:A2)); A2 only by C1.
	s := parse(t, "1\t=A1\t=sum(A1:A2)\n10\n")
	assert.Equal(t, []sheet.Address{addr(0, 1), addr(0, 2)}, s.Dependents(addr(0, 0)))
	assert.Equal(t, []sheet.Address{addr(0, 2)}, s.Dependents(addr(1, 0)))
}

func TestDependents_None(t *testing.T) {
	t.Parallel()

	// A cell no formula references has no dependents.
	assert.Nil(t, parse(t, "1\t2\t=A1\n").Dependents(addr(0, 1)))
}
