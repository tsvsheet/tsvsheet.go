package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsvsheet/tsvsheet.go/internal/session"
)

func TestView_DirtyAndDiagnostics(t *testing.T) {
	t.Parallel()

	s, err := session.New([]byte("=bogus(A1)\n")) // unknown func → diagnostic
	require.NoError(t, err)
	m := New(s, nil, nil)
	assert.Contains(t, stripANSI(m.View()), "diagnostic")

	m = press(t, m, "enter")
	m.buffer = "9" // replace the formula with a literal
	m = press(t, m, "enter")
	assert.Contains(t, stripANSI(m.View()), "unsaved")
}

func TestView_ErrorCircAndLongValues(t *testing.T) {
	t.Parallel()

	// #REF!, #CIRC!, and a long literal exercise the error and clip styling.
	s, err := session.New([]byte("=Z99\t=A2+1\t1234567890\n=B1+1\t5\t6\n"))
	require.NoError(t, err)
	view := stripANSI(New(s, nil, nil).View())
	assert.Contains(t, view, "#REF!")
	assert.Contains(t, view, "#CIRC!")
	assert.Contains(t, view, "…") // clipped long value
}

// TestGridRow_NeverRenumbersAroundAHiddenRow pins the claim that the gutter
// marks a skip rather than closing it up: a hidden row leaves a gap in the row
// numbers, so what the terminal shows still addresses the same cells the file
// does. Renumbering would make every A1 reference a lie on screen.
func TestGridRow_NeverRenumbersAroundAHiddenRow(t *testing.T) {
	t.Parallel()

	s, err := session.New([]byte("#.hide\trows(range(2:2))\na\nb\nc\n"))
	require.NoError(t, err)
	m := New(s, nil, nil)

	assert.Contains(t, m.gridRow(0), "1")
	assert.Contains(t, m.gridRow(2), "3", "row 3 keeps its number even with row 2 hidden")
}

// TestGutterStyle_MarksTheOneThingATerminalCannotShowByPosition pins why frozen
// rows are tinted at all: a pane that stays put while the rest scrolls has no
// counterpart in a printed grid, so the declaration would otherwise be
// invisible.
func TestGutterStyle_MarksTheOneThingATerminalCannotShowByPosition(t *testing.T) {
	t.Parallel()

	s, err := session.New([]byte("#.freeze\trows(count(1))\na\nb\n"))
	require.NoError(t, err)
	m := New(s, nil, nil)

	assert.NotEqual(t, m.gutterStyle(1).Render("2"), m.gutterStyle(0).Render("1"),
		"a frozen row's number is styled differently from an ordinary one")
}
