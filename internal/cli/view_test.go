package cli

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

// viewGrid is the grid the projection tests project.
var viewGrid = tsvsheet.Grid{
	{"name", "qty", "note"},
	{"widget", "3", "scratch"},
	{"gadget", "2", "scratch"},
}

// TestProjectKeepsHiddenByDefault proves the pipeline default: a hidden row is
// still data, so keeping it is what `render` does unless asked otherwise.
func TestProjectKeepsHiddenByDefault(t *testing.T) {
	t.Parallel()

	view := tsvsheet.View{
		HiddenRows: tsvsheet.Selection{2: true},
		HiddenCols: tsvsheet.Selection{3: true},
		HeaderRows: tsvsheet.Selection{1: true},
	}
	got, err := project(viewGrid, view, hiddenKeep)
	require.NoError(t, err)
	assert.Equal(t, viewGrid, got.grid, "nothing is dropped")
	assert.Equal(t, tsvsheet.Selection{1: true}, got.headerRows)
}

// TestProjectDropsAndRenumbers proves the projected artifact: hidden rows and
// columns go, and a surviving header row is renumbered to where it landed, so a
// format that marks headers still marks the right one.
func TestProjectDropsAndRenumbers(t *testing.T) {
	t.Parallel()

	view := tsvsheet.View{
		HiddenRows: tsvsheet.Selection{2: true},
		HiddenCols: tsvsheet.Selection{3: true},
		HeaderRows: tsvsheet.Selection{1: true},
	}
	got, err := project(viewGrid, view, hiddenDrop)
	require.NoError(t, err)
	assert.Equal(t, tsvsheet.Grid{{"name", "qty"}, {"gadget", "2"}}, got.grid)
	assert.Equal(t, tsvsheet.Selection{1: true}, got.headerRows)

	// A header row that is itself hidden takes its marking with it.
	hiddenHeader := tsvsheet.View{
		HiddenRows: tsvsheet.Selection{1: true},
		HiddenCols: tsvsheet.Selection{},
		HeaderRows: tsvsheet.Selection{1: true},
	}
	bare, err := project(viewGrid, hiddenHeader, hiddenDrop)
	require.NoError(t, err)
	assert.Empty(t, bare.headerRows)
	assert.Len(t, bare.grid, 2)
}

// TestProjectRejectsAnUnknownPolicy proves --hidden takes the two words it
// documents and nothing else.
func TestProjectRejectsAnUnknownPolicy(t *testing.T) {
	t.Parallel()

	_, err := project(viewGrid, tsvsheet.View{}, "maybe")
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrUnknownHiddenPolicy))
}

// TestWriteHTMLSections covers the three table shapes a header declaration can
// produce: none, a leading block with a body, and a table that is all header.
func TestWriteHTMLSections(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name    string
		headers tsvsheet.Selection
		says    string
		omits   string
	}{
		{
			name: "no declaration leaves a bare table", headers: tsvsheet.Selection{},
			says: "<tr><td>name</td>", omits: "<thead>",
		},
		{
			name: "a leading header block becomes a thead", headers: tsvsheet.Selection{1: true},
			says: "<thead>\n<tr><th>name</th>", omits: "",
		},
		{
			name: "a table that is all header has no tbody", headers: tsvsheet.Selection{1: true, 2: true, 3: true},
			says: "<thead>", omits: "<tbody>",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			require.NoError(t, writeHTML(&out, projection{grid: viewGrid, headerRows: c.headers}))
			assert.Contains(t, out.String(), c.says)
			if c.omits != "" {
				assert.NotContains(t, out.String(), c.omits)
			}
		})
	}
}

// TestLeadingHeaderRowsStopsAtTheGap proves a header declared further down the
// sheet still gets header cells but does not pull the rows above it into the
// <thead>, which would reorder the table.
func TestLeadingHeaderRowsStopsAtTheGap(t *testing.T) {
	t.Parallel()

	p := projection{grid: viewGrid, headerRows: tsvsheet.Selection{2: true}}
	assert.Equal(t, headerBlock(0), p.leadingHeaderRows())

	var out bytes.Buffer
	require.NoError(t, writeHTML(&out, p))
	assert.NotContains(t, out.String(), "<thead>")
	assert.Contains(t, out.String(), "<tr><th>widget</th>", "it is still a header row, in place")
}

// TestRunRenderHonoursHidden proves the flag reaches the output: the same sheet
// renders with its hidden column by default and without it on request.
func TestRunRenderHonoursHidden(t *testing.T) {
	t.Parallel()

	const src = "#.header\trows(count(1))\n#.hide\tcols(range(C:C))\nname\tqty\tnote\nwidget\t3\tscratch\n"

	kept, out, _ := streamsWith(src)
	require.NoError(t, runRender(kept, "-", formatTSV, hiddenKeep, false, tsvsheet.DefaultLimits(), nil, ""))
	assert.Equal(t, "name\tqty\tnote\nwidget\t3\tscratch\n", out.String())

	dropped, projected, _ := streamsWith(src)
	require.NoError(t, runRender(dropped, "-", formatTSV, hiddenDrop, false, tsvsheet.DefaultLimits(), nil, ""))
	assert.Equal(t, "name\tqty\nwidget\t3\n", projected.String())
}

// TestRunRenderRejectsAnUnknownHiddenPolicy proves a mistyped flag fails loudly
// rather than silently falling back to keeping or dropping.
func TestRunRenderRejectsAnUnknownHiddenPolicy(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("a\tb\n")
	err := runRender(streams, "-", formatTSV, "maybe", false, tsvsheet.DefaultLimits(), nil, "")
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrUnknownHiddenPolicy)
}

// TestCheckReportsDirectiveFindingsByLine proves check reads the `#.` lines a
// plain sheet drops, and addresses each finding where it lives: a cell finding
// by its cell, a directive finding by its line.
func TestCheckReportsDirectiveFindingsByLine(t *testing.T) {
	t.Parallel()

	streams, _, errOut := streamsWith("#.hide\trows(3)\nname\t=BADFN(1)\n")
	err := runCheck(streams, "-", tsvsheet.DefaultLimits())
	require.Error(t, err)

	assert.Contains(t, errOut.String(), "line 1: ", "a directive finding is addressed by its line")
	assert.Contains(t, errOut.String(), "range(3:3)", "and names the spelling it wanted")
	assert.Contains(t, errOut.String(), "B1: unknown function", "a cell finding is still addressed by its cell")
}
