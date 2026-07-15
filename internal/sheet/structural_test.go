package sheet_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/sheet"
)

// sourceAt reads the source text of the cell at a 0-based (row, col).
func sourceAt(t *testing.T, s sheet.Sheet, row, col int) string {
	t.Helper()
	g := s.Source()
	require.Less(t, row, len(g))
	require.Less(t, col, len(g[row]))
	return g[row][col]
}

func TestInsertRow_ShiftsReferencesAndInsertsBlank(t *testing.T) {
	t.Parallel()

	// B1 = A1 (row above the insert, unchanged); B2 = sum(A1:A2) (range whose
	// lower endpoint follows its data down).
	s := parse(t, "10\t=A1\n20\t=sum(A1:A2)\n")
	got := s.InsertRow(addr(1, 0)) // blank row before row index 1

	assert.Equal(t, "=A1", sourceAt(t, got, 0, 1))         // A1 is above the insert
	assert.Equal(t, "=sum(A1:A3)", sourceAt(t, got, 2, 1)) // range grew over the blank
	g := got.Compute()
	assert.Equal(t, "10", cellAt(t, g, 0, 1))
	assert.Equal(t, "30", cellAt(t, g, 2, 1)) // 10 + 0(blank) + 20
	assert.Equal(t, "", cellAt(t, g, 1, 0))   // the inserted row is empty
	assert.Equal(t, "", cellAt(t, g, 1, 1))
}

func TestInsertRow_PastEndAppendsBlank(t *testing.T) {
	t.Parallel()

	// An index beyond the grid clamps to the end: a trailing blank row of empty
	// cells (as wide as the widest row).
	got := parse(t, "1\n2\n").InsertRow(addr(9, 0))
	assert.Len(t, got.Source(), 3)
	assert.Equal(t, []string{""}, got.Source()[2])
}

func TestDeleteRow_ReferenceToDeletedBecomesRef(t *testing.T) {
	t.Parallel()

	// A3 = A2 (single ref to the deleted row) → #REF!; A4 = sum(A1:A2) (range
	// whose lower endpoint is deleted) shrinks to sum(A1:A1).
	s := parse(t, "10\n20\n=A2\n=sum(A1:A2)\n")
	got := s.DeleteRow(addr(1, 0)) // delete row A2

	assert.Equal(t, "=#REF!", sourceAt(t, got, 1, 0))      // old A3, ref deleted
	assert.Equal(t, "=sum(A1:A1)", sourceAt(t, got, 2, 0)) // range endpoint clamped
	g := got.Compute()
	assert.Equal(t, "#REF!", cellAt(t, g, 1, 0))
	assert.Equal(t, "10", cellAt(t, g, 2, 0)) // sum(A1:A1)
}

func TestDeleteRow_WholeRangeDeletedCollapses(t *testing.T) {
	t.Parallel()

	// B1 = sum(A2:A2) references only row A2; deleting that row collapses the
	// range argument to #REF! (lo > hi). The formula sits in row 0, which
	// survives; only the reference (not the whole call) becomes #REF!.
	got := parse(t, "10\t=sum(A2:A2)\n20\n").DeleteRow(addr(1, 0))
	assert.Equal(t, "=sum(#REF!)", sourceAt(t, got, 0, 1))
	assert.Equal(t, "#REF!", cellAt(t, got.Compute(), 0, 1))
}

func TestDeleteRow_EveryShiftBranch(t *testing.T) {
	t.Parallel()

	// Row 0 probes every remapping of a delete at row A2: a single ref above
	// (A1, unchanged), on (A2 → #REF!), and below (A3 → A2) the deletion; and
	// range endpoints below (A3:A3), straddling (A1:A3), and above (A1:A1) it.
	s := parse(t, "10\t=A1\t=A2\t=A3\t=sum(A3:A3)\t=sum(A1:A3)\t=sum(A1:A1)\n20\n30\n")
	got := s.DeleteRow(addr(1, 0))

	assert.Equal(t, "=A1", sourceAt(t, got, 0, 1))         // above → unchanged
	assert.Equal(t, "=#REF!", sourceAt(t, got, 0, 2))      // on → deleted
	assert.Equal(t, "=A2", sourceAt(t, got, 0, 3))         // below → shifts up
	assert.Equal(t, "=sum(A2:A2)", sourceAt(t, got, 0, 4)) // range below → shifts up
	assert.Equal(t, "=sum(A1:A2)", sourceAt(t, got, 0, 5)) // straddling → high shifts
	assert.Equal(t, "=sum(A1:A1)", sourceAt(t, got, 0, 6)) // above → unchanged
}

func TestDeleteRow_OutOfRangeIsNoOp(t *testing.T) {
	t.Parallel()

	s := parse(t, "1\n=A1\n")
	assert.Equal(t, s.Source(), s.DeleteRow(addr(9, 0)).Source())
	assert.Equal(t, s.Source(), s.DeleteRow(addr(-1, 0)).Source())
}

func TestInsertCol_ShiftsColumnReferences(t *testing.T) {
	t.Parallel()

	// C1 = A1 + B1: inserting a column before B pushes B1's data (and its
	// reference) to C, so B1 becomes C1 and C1 becomes D1.
	got := parse(t, "1\t2\t=A1 + B1\n").InsertCol(addr(0, 1))
	assert.Equal(t, "=A1 + C1", sourceAt(t, got, 0, 3))
	assert.Equal(t, "3", cellAt(t, got.Compute(), 0, 3)) // 1 + 2
}

func TestDeleteCol_ReferenceToDeletedBecomesRef(t *testing.T) {
	t.Parallel()

	// Deleting column B (the 2) makes =A1+B1 read a deleted cell → #REF!.
	got := parse(t, "1\t2\t=A1 + B1\n").DeleteCol(addr(0, 1))
	assert.Equal(t, "=A1 + #REF!", sourceAt(t, got, 0, 1))
	assert.Equal(t, "#REF!", cellAt(t, got.Compute(), 0, 1))
}

func TestDeleteCol_OutOfRangeIsNoOp(t *testing.T) {
	t.Parallel()

	s := parse(t, "1\t=A1\n")
	assert.Equal(t, s.Source(), s.DeleteCol(addr(0, 9)).Source())
}

func TestStructuralEdits_RaggedRowsAndAllNodeForms(t *testing.T) {
	t.Parallel()

	// A ragged grid: row 0 reaches column C, row 1 has a single short cell.
	// The formula exercises every mapRefs branch — unary, percent, binary,
	// call, and a bare literal (default) — so a column edit rewrites through
	// all of them without disturbing the literal.
	s := parse(t, "1\t=-A1 + sum(A1:A1) & B1% & \"x\"\t7\n9\n")
	const formula = "=-A1 + sum(A1:A1) & B1% & \"x\""

	ins := s.InsertCol(addr(0, 2))                   // insert before column C; short row 1 cannot reach it
	assert.Len(t, ins.Source()[1], 1)                // ragged row untouched
	assert.Equal(t, formula, sourceAt(t, ins, 0, 1)) // no ref is at/after C, so unchanged
	assert.Len(t, ins.Source()[0], 4)                // a blank column was spliced in

	del := s.DeleteCol(addr(0, 2))                   // drop column C (the 7); short row 1 cannot reach it
	assert.Len(t, del.Source()[1], 1)                // ragged row untouched
	assert.Len(t, del.Source()[0], 2)                // row 0 now ends at the formula
	assert.Equal(t, formula, sourceAt(t, del, 0, 1)) // literal "x" and all nodes preserved
}
