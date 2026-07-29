package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsvsheet/tsvsheet.go/internal/session"
)

func TestTUIHidesWhatTheSheetHides(t *testing.T) {
	t.Parallel()

	s, err := session.New([]byte(
		"#.hide\tcols(range(B:B))\n#.hide\trows(range(2:2))\nname\tscratch\ndrop\tx\nkeep\ty\n",
	))
	require.NoError(t, err)

	view := stripANSI(New(s, nil, nil).View())
	assert.Contains(t, view, "name")
	assert.NotContains(t, view, "scratch", "the hidden column is not drawn")
	assert.NotContains(t, view, "drop", "the hidden row is not drawn")
	assert.Contains(t, view, "⋯3", "and the gutter marks where rows were skipped")
}

func TestTUIRevealShowsWithoutUnhiding(t *testing.T) {
	t.Parallel()

	const src = "#.hide\tcols(range(B:B))\nname\tscratch\nwidget\tx\n"
	s, err := session.New([]byte(src))
	require.NoError(t, err)

	revealed := press(t, New(s, nil, nil), "v")
	assert.Contains(t, stripANSI(revealed.View()), "scratch")
	assert.Contains(t, stripANSI(revealed.View()), "Showing hidden")
	assert.Equal(t, src, string(s.Source()), "revealing never writes to the file")

	rehidden := press(t, revealed, "v")
	assert.NotContains(t, stripANSI(rehidden.View()), "scratch")
}

func TestTUIRevealSaysSoWhenThereIsNothingToReveal(t *testing.T) {
	t.Parallel()

	s, err := session.New([]byte("a\tb\n1\t2\n"))
	require.NoError(t, err)

	assert.Contains(t, stripANSI(press(t, New(s, nil, nil), "v").View()), "hides nothing")
}

func TestTUIMarksHeaderAndFrozenRows(t *testing.T) {
	t.Parallel()

	s, err := session.New([]byte(
		"#.header\trows(count(1))\n#.freeze\trows(count(1))\nname\tqty\nwidget\t3\n",
	))
	require.NoError(t, err)

	m := New(s, nil, nil)
	assert.True(t, m.headerRow(0))
	assert.True(t, m.frozenRow(0))
	assert.False(t, m.headerRow(1))
	assert.NotEmpty(t, stripANSI(m.View()))
}

func TestTUIRehidingRescuesTheCursor(t *testing.T) {
	t.Parallel()

	// Hidden rows 2-3 of 4: a cursor on row 2 moves down to the visible row 4.
	down, err := session.New([]byte("#.hide\trows(range(2:3))\na\nb\nc\nd\n"))
	require.NoError(t, err)
	m := New(down, nil, nil)
	m.isRevealing, m.row = true, 1
	assert.Equal(t, 3, m.toggleReveal().row)

	// Hidden rows 2-4 of 4: nothing below is visible, so it moves back to row 1.
	up, err := session.New([]byte("#.hide\trows(range(2:4))\na\nb\nc\nd\n"))
	require.NoError(t, err)
	tail := New(up, nil, nil)
	tail.isRevealing, tail.row = true, 3
	assert.Equal(t, 0, tail.toggleReveal().row)

	// The column axis behaves the same, including the fall back to the last
	// visible column when the cursor sits beyond every one of them.
	cols, err := session.New([]byte("#.hide\tcols(range(B:C))\na\tb\tc\n"))
	require.NoError(t, err)
	wide := New(cols, nil, nil)
	wide.isRevealing, wide.col = true, 2
	assert.Equal(t, 0, wide.toggleReveal().col)

	// A sheet that hides every row is the same case on the other axis.
	allRows, err := session.New([]byte("#.hide\trows(range(1:2))\na\nb\n"))
	require.NoError(t, err)
	empty := New(allRows, nil, nil)
	empty.isRevealing, empty.row = true, 1
	assert.Equal(t, 1, empty.toggleReveal().row)

	// A sheet that hides every column has nowhere to put the cursor, so it
	// stays where it is rather than being moved to a column that is not there.
	all, err := session.New([]byte("#.hide\tcols(range(A:B))\na\tb\n"))
	require.NoError(t, err)
	blank := New(all, nil, nil)
	blank.isRevealing, blank.col = true, 1
	assert.Equal(t, 1, blank.toggleReveal().col)
}

// TestOnVisibleRow_ReHidingNeverStrandsTheCursor pins the rescue the doc names:
// after reveal is switched back off, a cursor parked on a row that just
// disappeared must be pulled onto one that is actually rendered — otherwise the
// cursor addresses a cell the user cannot see and every subsequent edit lands
// somewhere unexpected.
func TestOnVisibleRow_ReHidingNeverStrandsTheCursor(t *testing.T) {
	t.Parallel()

	s, err := session.New([]byte("#.hide\trows(range(2:3))\na\nb\nc\nd\n"))
	require.NoError(t, err)
	m := New(s, nil, nil)

	assert.Equal(t, 0, m.onVisibleRow(0), "an already-visible row is left alone")
	assert.Equal(t, 3, m.onVisibleRow(1), "a hidden row moves down to the next visible one")
	assert.Equal(t, 3, m.onVisibleRow(2))
}

// TestToggleReveal_IsSessionStateAndNeverAnEdit pins that revealing hidden rows
// does not touch the file: the sheet still carries the view its author
// declared, so quitting after a reveal cannot silently rewrite the directive.
func TestToggleReveal_IsSessionStateAndNeverAnEdit(t *testing.T) {
	t.Parallel()

	const src = "#.hide\trows(range(2:2))\na\nb\nc\n"
	s, err := session.New([]byte(src))
	require.NoError(t, err)
	m := New(s, nil, nil)

	revealed := m.toggleReveal()
	assert.NotEqual(t, m.isRevealing, revealed.isRevealing, "the flag flipped")
	assert.Equal(t, src, string(revealed.session.Source()), "the file is byte-identical")
}
