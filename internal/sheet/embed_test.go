package sheet_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/sheet"
)

// errNoSheet is what the in-memory loader returns for an unknown reference.
var errNoSheet = errors.New("no such sheet")

// memLoader is an in-memory Loader over a name→source map; the resolved
// path is the reference itself (a flat namespace), which is enough to exercise
// embedding and cycle detection without a filesystem.
func memLoader(sheets map[string]string) sheet.Loader {
	return func(_, ref sheet.Path) (sheet.Sheet, sheet.Path, error) {
		src, ok := sheets[string(ref)]
		if !ok {
			return sheet.Sheet{}, "", errNoSheet
		}
		s, err := sheet.Parse([]byte(src))
		return s, ref, err
	}
}

// embedGrid parses root and computes it with the loader and a base path.
func embedGrid(t *testing.T, root string, sheets map[string]string) sheet.Grid {
	t.Helper()
	s, err := sheet.Parse([]byte(root))
	require.NoError(t, err)
	return s.ComputeWith(sheet.ComputeOptions{Loader: memLoader(sheets), Base: "root"})
}

func TestEmbed_OutputValueFlowsIntoCell(t *testing.T) {
	t.Parallel()

	// The root's A1 embeds "child", whose OUTPUT cell sums a column.
	g := embedGrid(t, "=sheet(\"child\")\n", map[string]string{
		"child": "1\n2\n3\n=output(sum(A1:A3))\n",
	})
	assert.Equal(t, "6", cellAt(t, g, 0, 0))
}

func TestEmbed_InputsParameteriseTheSubSheet(t *testing.T) {
	t.Parallel()

	// SHEET passes two arguments; the child reads them with INPUT and outputs
	// their sum — a spreadsheet used as a function.
	g := embedGrid(t, "=sheet(\"add\", 10, 20)\n", map[string]string{
		"add": "=output(input(1) + input(2))\n",
	})
	assert.Equal(t, "30", cellAt(t, g, 0, 0))
}

func TestEmbed_NestedSubSheets(t *testing.T) {
	t.Parallel()

	// root embeds "outer", which itself embeds "inner"; values flow up the chain.
	g := embedGrid(t, "=sheet(\"outer\")\n", map[string]string{
		"outer": "=output(sheet(\"inner\") * 2)\n",
		"inner": "=output(21)\n",
	})
	assert.Equal(t, "42", cellAt(t, g, 0, 0))
}

func TestEmbed_NoLoaderIsRef(t *testing.T) {
	t.Parallel()

	// A plain compute has no loader, so SHEET cannot resolve → #REF!.
	assert.Equal(t, "#REF!", cellAt(t, compute(t, "=sheet(\"child\")\n"), 0, 0))
}

func TestEmbed_FailureModes(t *testing.T) {
	t.Parallel()

	sheets := map[string]string{
		"noout":  "1\n2\n",                   // no OUTPUT cell
		"twoout": "=output(1)\t=output(2)\n", // two OUTPUT cells
		"needs":  "=output(input(1))\n",      // needs an argument
		"badidx": "=output(input(\"x\"))\n",  // non-numeric INPUT index
	}
	cases := map[string]string{
		"=sheet()":            string(sheet.ErrValue), // SHEET arity
		"=sheet(1/0)":         string(sheet.ErrDiv),   // path expression errors through
		"=sheet(\"missing\")": string(sheet.ErrRef),   // loader cannot resolve
		"=sheet(\"noout\")":   string(sheet.ErrRef),   // no OUTPUT cell
		"=sheet(\"twoout\")":  string(sheet.ErrRef),   // ambiguous OUTPUT
		"=sheet(\"needs\")":   string(sheet.ErrRef),   // INPUT(1) with no args
		"=sheet(\"badidx\")":  string(sheet.ErrValue), // INPUT with a text index
	}
	for expr, want := range cases {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, cellAt(t, embedGrid(t, expr+"\n", sheets), 0, 0))
		})
	}
}

func TestEmbed_InputArityAndOutOfRange(t *testing.T) {
	t.Parallel()

	// INPUT with the wrong arity is #VALUE!; an out-of-range index is #REF!.
	sheets := map[string]string{
		"arity": "=output(input())\n",  // INPUT needs exactly one argument
		"range": "=output(input(5))\n", // only one argument is passed
	}
	assert.Equal(t, "#VALUE!", cellAt(t, embedGrid(t, "=sheet(\"arity\", 1)\n", sheets), 0, 0))
	assert.Equal(t, "#REF!", cellAt(t, embedGrid(t, "=sheet(\"range\", 1)\n", sheets), 0, 0))
}

func TestEmbed_CycleIsCirc(t *testing.T) {
	t.Parallel()

	// root embeds "child", which embeds "root" back — a cross-sheet cycle.
	g := embedGrid(t, "=sheet(\"child\")\n", map[string]string{
		"root":  "=sheet(\"child\")\n",
		"child": "=output(sheet(\"root\"))\n",
	})
	assert.Equal(t, "#CIRC!", cellAt(t, g, 0, 0))
}

func TestEmbed_CheckAcceptsBuiltins(t *testing.T) {
	t.Parallel()

	// SHEET, INPUT, and OUTPUT are known functions — Check must not flag them.
	s, err := sheet.Parse([]byte("=sheet(\"x\", input(1))\t=output(A1)\n"))
	require.NoError(t, err)
	assert.Empty(t, sheet.Check(s))
}

func TestEmbeddedGrid_ReturnsSubSheet(t *testing.T) {
	t.Parallel()

	// The embed cell's sub-sheet computes with the passed argument; its whole
	// grid is returned for nested rendering.
	s, err := sheet.Parse([]byte("=sheet(\"child\", 5)\n"))
	require.NoError(t, err)
	path, grid, ok := s.EmbeddedGrid(addr(0, 0), sheet.ComputeOptions{
		Loader: memLoader(map[string]string{"child": "=input(1)\t=output(A1 * 2)\n"}),
		Base:   "root",
	})
	require.True(t, ok)
	assert.Equal(t, sheet.Path("child"), path)
	assert.Equal(t, "5", grid[0][0])  // A1 = INPUT(1)
	assert.Equal(t, "10", grid[0][1]) // B1 = OUTPUT(A1*2)
}

func TestEmbeddedGrid_NotAnEmbed(t *testing.T) {
	t.Parallel()

	s, err := sheet.Parse([]byte("hi\t=A1\n"))
	require.NoError(t, err)

	_, _, litOK := s.EmbeddedGrid(addr(0, 0), sheet.ComputeOptions{})
	assert.False(t, litOK) // a literal
	_, _, refOK := s.EmbeddedGrid(addr(0, 1), sheet.ComputeOptions{})
	assert.False(t, refOK) // a formula, but not a SHEET call
	_, _, offOK := s.EmbeddedGrid(addr(9, 9), sheet.ComputeOptions{})
	assert.False(t, offOK) // off the grid
}

func TestEmbeddedGrid_UnresolvedIsNotOK(t *testing.T) {
	t.Parallel()

	s, err := sheet.Parse([]byte("=sheet(\"missing\")\n"))
	require.NoError(t, err)
	_, _, ok := s.EmbeddedGrid(addr(0, 0), sheet.ComputeOptions{
		Loader: memLoader(map[string]string{}),
		Base:   "root",
	})
	assert.False(t, ok)
}
