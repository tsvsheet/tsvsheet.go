package session_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tsvsheet/go-tsvsheet"
)

func TestInsertRow_GrowsAndDirties(t *testing.T) {
	t.Parallel()

	s := newSession(t)
	s.InsertRow(tsvsheet.Address{Row: 1})
	st := s.Snapshot()
	assert.Len(t, st.Source, 4) // 3 rows → 4
	assert.True(t, st.IsDirty)
}

func TestDeleteRow_Shrinks(t *testing.T) {
	t.Parallel()

	s := newSession(t)
	s.DeleteRow(tsvsheet.Address{Row: 1})
	assert.Len(t, s.Snapshot().Source, 2)
}

func TestInsertCol_Widens(t *testing.T) {
	t.Parallel()

	s := newSession(t)
	s.InsertCol(tsvsheet.Address{Col: 1})
	assert.Len(t, s.Snapshot().Source[0], 5) // 4 cols → 5
}

func TestDeleteCol_Narrows(t *testing.T) {
	t.Parallel()

	s := newSession(t)
	s.DeleteCol(tsvsheet.Address{Col: 1})
	assert.Len(t, s.Snapshot().Source[0], 3)
}

func TestFill_CopiesWithRebasedReferences(t *testing.T) {
	t.Parallel()

	// Fill D2 (=B2+C2) into D3: the copy rebases to that row's cells.
	s := newSession(t)
	d3 := tsvsheet.Address{Row: 2, Col: 3}
	s.Fill(tsvsheet.Address{Row: 1, Col: 3}, tsvsheet.Span{From: d3, To: d3})
	st := s.Snapshot()
	assert.Equal(t, "=B3 + C3", st.Source[2][3])
	assert.Equal(t, "9", st.Computed[2][3]) // 4 + 5
	assert.True(t, st.IsDirty)
}

func TestDuplicateRow_AddsRebasedRow(t *testing.T) {
	t.Parallel()

	// Duplicating row 2 rebases the duplicate's formula and shifts the row
	// below it, exactly as the engine's insert does.
	s := newSession(t)
	s.DuplicateRow(tsvsheet.Address{Row: 1})
	st := s.Snapshot()
	assert.Len(t, st.Source, 4)
	assert.Equal(t, "=B3 + C3", st.Source[2][3]) // the duplicate
	assert.Equal(t, "=B4 + C4", st.Source[3][3]) // the shifted original
	assert.Equal(t, "5", st.Computed[2][3])
	assert.True(t, st.IsDirty)
}

func TestDuplicateCol_AddsRebasedColumn(t *testing.T) {
	t.Parallel()

	// Duplicating column B: the duplicate carries B's literals, and the D
	// formulas' C references follow their shifted data.
	s := newSession(t)
	s.DuplicateCol(tsvsheet.Address{Col: 1})
	st := s.Snapshot()
	assert.Len(t, st.Source[0], 5)
	assert.Equal(t, "2", st.Source[1][2])        // duplicated literal
	assert.Equal(t, "=B2 + D2", st.Source[1][4]) // C followed its data to D
	assert.Equal(t, "5", st.Computed[1][4])
	assert.True(t, st.IsDirty)
}
