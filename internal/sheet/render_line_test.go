package sheet_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/sheet"
	"github.com/uplang/tsvsheet.go/internal/tsvt"
)

// lineOf parses a one-line template and returns its single line.
func lineOf(t *testing.T, src string) tsvt.Line {
	t.Helper()
	tmpl, err := tsvt.Parse(tsvt.Source(src))
	require.NoError(t, err)
	require.Len(t, tmpl.Lines, 1)
	return tmpl.Lines[0]
}

func TestLineKindOf(t *testing.T) {
	t.Parallel()

	cases := map[string]sheet.LineKind{
		"=header(1)": sheet.KindHeader,
		"=body":      sheet.KindBody,
		"=final":     sheet.KindFinal,
		"=A<":        sheet.KindStructural,
		"A\tB":       sheet.KindRow,
	}
	for src, want := range cases {
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, sheet.LineKindOf(lineOf(t, src)))
		})
	}
}

func TestRenderLine(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"=header(2)": "=header(2)",
		"=body":      "=body",
		"=final":     "=final",
		"=A<":        "=A<",
		"=C!":        "=C!",
		"E=C + D":    "E=C + D",
		"A$+1=Total": "A$+1=Total",
		"=sum(A:D)":  "=sum(A:D)",
		"C!":         "C!",
		`A="x"`:      `A="x"`,
	}
	for src, want := range cases {
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, sheet.RenderLine(lineOf(t, src)))
		})
	}
}

func TestRenderLine_Row(t *testing.T) {
	t.Parallel()

	// A multi-cell row round-trips its TAB-joined cell sources.
	assert.Equal(t, "A\t=C + D\tTotal", sheet.RenderLine(lineOf(t, "A\t=C + D\tTotal")))
}

func TestRenderCell(t *testing.T) {
	t.Parallel()

	tmpl, err := tsvt.Parse(tsvt.Source("\t=C + D\tTotal\tE=sum(A:B)\tC!"))
	require.NoError(t, err)
	row, ok := tmpl.Lines[0].(tsvt.Row)
	require.True(t, ok)

	got := make([]string, len(row.Cells))
	for i, cell := range row.Cells {
		got[i] = sheet.RenderCell(cell)
	}
	assert.Equal(t, []string{"", "=C + D", "Total", "E=sum(A:B)", "C!"}, got)
}

func TestRenderCell_Unary(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "=-C", sheet.RenderCell(firstRowCell(t, "=-C")))
	assert.Equal(t, "=3.5", sheet.RenderCell(firstRowCell(t, "=3.5")))
}

// firstRowCell parses a one-cell row and returns the cell.
func firstRowCell(t *testing.T, src string) tsvt.Cell {
	t.Helper()
	row, ok := lineOf(t, src).(tsvt.Row)
	require.True(t, ok)
	require.Len(t, row.Cells, 1)
	return row.Cells[0]
}
