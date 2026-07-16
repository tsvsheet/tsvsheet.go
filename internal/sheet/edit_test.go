package sheet_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/constants"
	"github.com/uplang/tsvsheet.go/internal/sheet"
)

func TestCells_ProjectsNonEmpty(t *testing.T) {
	t.Parallel()

	s, err := sheet.Parse([]byte("a\t\t=A1\n"))
	require.NoError(t, err)

	cells := s.Cells()
	require.Len(t, cells, 2) // the empty middle cell is omitted

	assert.Equal(t, "A1", cells[0].Address.String())
	assert.Equal(t, "a", cells[0].Text)
	assert.False(t, cells[0].IsFormula)

	assert.Equal(t, "C1", cells[1].Address.String())
	assert.Equal(t, "=A1", cells[1].Text)
	assert.True(t, cells[1].IsFormula)
}

func TestSet_LiteralInPlace(t *testing.T) {
	t.Parallel()

	s, err := sheet.Parse([]byte("1\t2\n"))
	require.NoError(t, err)

	next, err := s.Set(sheet.Address{Row: 0, Col: 0}, "9", sheet.DefaultLimits())
	require.NoError(t, err)

	// The new sheet reflects the edit; the original is unchanged (immutable Set).
	assert.Equal(t, "9", next.Source()[0][0])
	assert.Equal(t, "1", s.Source()[0][0])
	assert.Equal(t, "9", next.Compute()[0][0])
}

func TestSet_FormulaComputes(t *testing.T) {
	t.Parallel()

	s, err := sheet.Parse([]byte("2\t3\n"))
	require.NoError(t, err)

	next, err := s.Set(sheet.Address{Row: 0, Col: 1}, "=A1*10", sheet.DefaultLimits())
	require.NoError(t, err)
	assert.Equal(t, "=A1*10", next.Source()[0][1])
	assert.Equal(t, "20", next.Compute()[0][1])
}

func TestSet_GrowsGrid(t *testing.T) {
	t.Parallel()

	s, err := sheet.Parse([]byte("1\t2\n"))
	require.NoError(t, err)

	// Write well beyond the current bounds: new rows and new columns appear,
	// padded with empty cells.
	next, err := s.Set(sheet.Address{Row: 2, Col: 3}, "x", sheet.DefaultLimits())
	require.NoError(t, err)

	src := next.Source()
	require.Len(t, src, 3)
	assert.Equal(t, "x", src[2][3])
	assert.Equal(t, "", src[2][0])  // padded within the grown row
	assert.Equal(t, "1", src[0][0]) // original row preserved
	assert.Empty(t, src[1])         // intervening row stays empty
}

func TestSet_MalformedFormulaIsSyntaxError(t *testing.T) {
	t.Parallel()

	s, err := sheet.Parse([]byte("1\n"))
	require.NoError(t, err)

	_, err = s.Set(sheet.Address{Row: 0, Col: 0}, "=sum(", sheet.DefaultLimits())
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrSyntax)
}

func TestSet_KeepsOtherRows(t *testing.T) {
	t.Parallel()

	// A grid taller than the edited row: growCells keeps the existing row count
	// (maxInt returns the source length, not row+1).
	s, err := sheet.Parse([]byte("1\n2\n3\n"))
	require.NoError(t, err)

	next, err := s.Set(sheet.Address{Row: 0, Col: 0}, "9", sheet.DefaultLimits())
	require.NoError(t, err)

	src := next.Source()
	require.Len(t, src, 3)
	assert.Equal(t, "9", src[0][0])
	assert.Equal(t, "2", src[1][0])
	assert.Equal(t, "3", src[2][0])
}

func TestSet_RejectsNegativeAddress(t *testing.T) {
	t.Parallel()

	s, err := sheet.Parse([]byte("1\n"))
	require.NoError(t, err)

	_, err = s.Set(sheet.Address{Row: -1, Col: 0}, "x", sheet.DefaultLimits())
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrInvalidValue)

	// An address beyond the grid limit is rejected before growing (OOM guard).
	_, err = s.Set(sheet.Address{Row: 2_000_000, Col: 0}, "x", sheet.DefaultLimits())
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrInvalidValue)
	_, err = s.Set(sheet.Address{Row: 0, Col: 2_000_000}, "x", sheet.DefaultLimits())
	assert.ErrorIs(t, err, constants.ErrInvalidValue)
}

func TestSource_RoundTrips(t *testing.T) {
	t.Parallel()

	s, err := sheet.Parse([]byte("a\t=A1\n"))
	require.NoError(t, err)

	src := s.Source()
	assert.Equal(t, "a", src[0][0])
	assert.Equal(t, "=A1", src[0][1]) // formula source kept verbatim
}
