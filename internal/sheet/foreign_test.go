package sheet_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/sheet"
)

func TestForeign_SingleCell(t *testing.T) {
	t.Parallel()

	// A1 reads cell A2 of another sheet by name.
	g := embedGrid(t, "=\"config.tsvt\"!A2 * 100\n", map[string]string{
		"config.tsvt": "rate\n0.2\n",
	})
	assert.Equal(t, "20", cellAt(t, g, 0, 0))
}

func TestForeign_RangeAggregate(t *testing.T) {
	t.Parallel()

	// A cross-sheet range flows through argCells into an aggregate.
	g := embedGrid(t, "=sum(\"nums.tsvt\"!A1:A3)\n", map[string]string{
		"nums.tsvt": "1\n2\n3\n",
	})
	assert.Equal(t, "6", cellAt(t, g, 0, 0))
}

func TestForeign_RangeMatrixLookup(t *testing.T) {
	t.Parallel()

	// A lookup reads a cross-sheet range as a 2-D matrix (the foreignMatrix path).
	g := embedGrid(t, "=index(\"grid.tsvt\"!A1:C1, 1, 2)\n", map[string]string{
		"grid.tsvt": "10\t20\t30\n",
	})
	assert.Equal(t, "20", cellAt(t, g, 0, 0))
}

func TestForeign_Transitive(t *testing.T) {
	t.Parallel()

	// The foreign cell is itself a formula reading a third sheet.
	g := embedGrid(t, "=\"mid.tsvt\"!A1\n", map[string]string{
		"mid.tsvt":  "=\"leaf.tsvt\"!A1 + 1\n",
		"leaf.tsvt": "41\n",
	})
	assert.Equal(t, "42", cellAt(t, g, 0, 0))
}

func TestForeign_NoLoaderIsRef(t *testing.T) {
	t.Parallel()

	// A plain compute has no loader, so a cross-sheet reference is #REF!.
	assert.Equal(t, "#REF!", cellAt(t, compute(t, "=\"x.tsvt\"!A1\n"), 0, 0))
}

func TestForeign_ErrorModes(t *testing.T) {
	t.Parallel()

	sheets := map[string]string{"there.tsvt": "1\t2\n"}
	cases := map[string]string{
		"=\"missing.tsvt\"!A1":               string(sheet.ErrRef), // single, unresolved
		"=sum(\"missing.tsvt\"!A1:A3)":       string(sheet.ErrRef), // range, unresolved
		"=index(\"missing.tsvt\"!A1:B2,1,1)": string(sheet.ErrRef), // matrix, unresolved
	}
	for expr, want := range cases {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, cellAt(t, embedGrid(t, expr+"\n", sheets), 0, 0))
		})
	}
}

func TestForeign_CycleIsCirc(t *testing.T) {
	t.Parallel()

	// root reads a.tsvt!A1, which reads back the root sheet — a cross-sheet cycle.
	g := embedGrid(t, "=\"a.tsvt\"!A1\n", map[string]string{
		"a.tsvt": "=\"root\"!A1\n",
		"root":   "placeholder\n", // the loader must resolve "root" for the cycle to be seen
	})
	assert.Equal(t, "#CIRC!", cellAt(t, g, 0, 0))
}

func TestForeign_NotAPrecedent(t *testing.T) {
	t.Parallel()

	// B1 = A1 + "x.tsvt"!C1: the cross-sheet reference is not a local span, so
	// only A1 is a precedent.
	s := parse(t, "1\t=A1 + \"x.tsvt\"!C1\n")
	spans := s.Precedents(addr(0, 1))
	require.Len(t, spans, 1)
	assert.Equal(t, addr(0, 0), spans[0].From) // A1 only; the foreign ref is skipped
}

func TestForeign_StructuralEditsDoNotShift(t *testing.T) {
	t.Parallel()

	// Inserting a row must not shift a cross-sheet reference (it addresses
	// another sheet), while a local reference below the insert does shift.
	s := parse(t, "=\"x.tsvt\"!A5\t=A3\n0\n0\n")
	got := s.InsertRow(addr(0, 0))                            // insert a row at the top
	assert.Equal(t, "=\"x.tsvt\"!A5", sourceAt(t, got, 1, 0)) // foreign ref unchanged
	assert.Equal(t, "=A4", sourceAt(t, got, 1, 1))            // local ref shifted A3→A4
}
