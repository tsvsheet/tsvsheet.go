package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNavigation(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	m = press(t, m, "down")
	m = press(t, m, "right")
	assert.Equal(t, 1, m.row)
	assert.Equal(t, 1, m.col)

	m = press(t, m, "k") // up (vim)
	m = press(t, m, "h") // left (vim)
	assert.Equal(t, 0, m.row)
	assert.Equal(t, 0, m.col)
}

func TestNavigation_ClampsAtEdges(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	m = press(t, m, "up")   // already at top
	m = press(t, m, "left") // already at left
	assert.Equal(t, 0, m.row)
	assert.Equal(t, 0, m.col)

	for i := 0; i < 20; i++ {
		m = press(t, m, "down")
		m = press(t, m, "right")
	}
	assert.Equal(t, m.height()-1, m.row)
	assert.Equal(t, m.width()-1, m.col)
}

func TestEditCell_Literal(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	m = press(t, m, "right") // B1 (a literal "2")
	m = press(t, m, "enter") // edit
	assert.Equal(t, modeEdit, m.mode)

	m = press(t, m, "backspace") // clear "2"
	m = press(t, m, "9")
	m = press(t, m, "enter") // commit
	assert.Equal(t, modeNav, m.mode)

	state := m.state
	assert.Equal(t, "9", state.Source[0][1])
	assert.True(t, state.IsDirty)
}

func TestEditCell_EntersWithI(t *testing.T) {
	t.Parallel()

	m := press(t, newModel(t, nil), "i")
	assert.Equal(t, modeEdit, m.mode)
	assert.Equal(t, "name", m.buffer) // seeded with the cell's source
}

func TestEditCell_Space(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	m = press(t, m, "enter")
	m = press(t, m, "space")
	assert.Contains(t, m.buffer, " ")
}

func TestEditCell_Cancel(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	m = press(t, m, "enter")
	m = press(t, m, "5")
	m = press(t, m, "esc") // cancel
	assert.Equal(t, modeNav, m.mode)
	assert.Equal(t, "name", m.state.Source[0][0]) // unchanged
}

func TestEditCell_FormulaSyntaxErrorStaysEditing(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	m = press(t, m, "enter")
	m.buffer = "=sum(" // a malformed formula
	m = press(t, m, "enter")
	assert.Equal(t, modeEdit, m.mode) // stays so the buffer is not lost
	assert.NotEmpty(t, m.status)
}

func TestEditBuffer_UnhandledAndEmptyBackspace(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	m = press(t, m, "right") // B1, buffer seed "2"
	m = press(t, m, "enter")
	m = press(t, m, "backspace") // ""
	m = press(t, m, "backspace") // backspace on empty → no-op
	assert.Empty(t, m.buffer)
	m = press(t, m, "up") // unhandled key in edit mode → buffer unchanged
	assert.Empty(t, m.buffer)
}

func TestRefreshImports_Key(t *testing.T) {
	t.Parallel()

	// R refreshes imports (a recompute here, since the sheet has none) and reports
	// it, leaving the model in navigation mode.
	m := press(t, newModel(t, nil), "R")
	assert.Equal(t, "Imports refreshed.", m.status)
	assert.Equal(t, modeNav, m.mode)
}

func TestQuit_Clean(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	next, cmd := m.Update(keyMsg("q"))
	assert.True(t, next.(Model).isQuitting)
	assert.NotNil(t, cmd) // tea.Quit
}

func TestQuit_DirtyWarnsThenQuits(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	m = press(t, m, "enter")
	m = press(t, m, "9")
	m = press(t, m, "enter") // dirty

	m = press(t, m, "q") // first q → warn
	assert.False(t, m.isQuitting)
	assert.True(t, m.isConfirmingQuit)
	assert.Contains(t, m.status, "Unsaved")

	next, cmd := m.Update(keyMsg("q")) // second q → quit
	assert.True(t, next.(Model).isQuitting)
	assert.NotNil(t, cmd)
}

func TestQuit_DirtyThenMovementResets(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	m = press(t, m, "enter")
	m = press(t, m, "9")
	m = press(t, m, "enter")
	m = press(t, m, "q")    // warn
	m = press(t, m, "down") // movement resets confirm
	assert.False(t, m.isConfirmingQuit)
}

func TestQuit_CtrlCOnDirty(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	m = press(t, m, "enter")
	m = press(t, m, "9")
	m = press(t, m, "enter")
	m = press(t, m, "q")      // warn
	m = press(t, m, "ctrl+c") // ctrl+c after warn → quits
	assert.True(t, m.isQuitting)
}

func TestQuit_EscQuitsClean(t *testing.T) {
	t.Parallel()

	m := press(t, newModel(t, nil), "esc")
	assert.True(t, m.isQuitting)
}

func TestUnhandledKeyResetsConfirm(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	m = press(t, m, "z") // unknown nav key
	assert.False(t, m.isConfirmingQuit)
	assert.Equal(t, modeNav, m.mode)
}

// otherMsg is a message the update loop does not recognize.
type otherMsg struct{}

func TestView_NavAndEdit(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	view := stripANSI(m.View())
	assert.Contains(t, view, "tsvsheet")
	assert.Contains(t, view, "A1:") // formula bar addresses the cursor

	editing := press(t, m, "enter")
	assert.Contains(t, stripANSI(editing.View()), "▏") // edit caret in the formula bar

	quit := press(t, m, "q")
	assert.Empty(t, quit.View())
}

func TestFillKeys(t *testing.T) {
	t.Parallel()

	// ctrl+d on D2 fills from D1 (=B1+C1 → =B2 + C2, overwriting in place);
	// ctrl+r on B1's right neighbor fills the literal from the left.
	m := newModel(t, nil)
	m = press(t, m, "down")
	for range 3 {
		m = press(t, m, "right") // to D2
	}
	m = press(t, m, "ctrl+d")
	assert.Equal(t, "=B2 + C2", m.state.Source[1][3])
	assert.Equal(t, "Filled.", m.status)

	m2 := newModel(t, nil)
	m2 = press(t, m2, "right") // to B1
	m2 = press(t, m2, "ctrl+r")
	assert.Equal(t, "name", m2.state.Source[0][1])
}

func TestFillKeys_NoNeighborIsNoOp(t *testing.T) {
	t.Parallel()

	// The top row has nothing above and the first column nothing to the left.
	m := newModel(t, nil)
	m = press(t, m, "ctrl+d")
	m = press(t, m, "ctrl+r")
	assert.Equal(t, "name", m.state.Source[0][0])
	assert.False(t, m.state.IsDirty)
}

func TestDuplicateKeys(t *testing.T) {
	t.Parallel()

	// D duplicates the selected row below itself; C the selected column to its
	// right — both rebasing the duplicate's references.
	m := newModel(t, nil)
	m = press(t, m, "down")
	m = press(t, m, "D")
	assert.Len(t, m.state.Source, 3)
	assert.Equal(t, "=B3 + C3", m.state.Source[2][3])
	assert.Equal(t, "Row duplicated.", m.status)

	m2 := newModel(t, nil)
	m2 = press(t, m2, "right")
	m2 = press(t, m2, "C")
	assert.Len(t, m2.state.Source[0], 5)
	assert.Equal(t, "2", m2.state.Source[0][2])
	assert.Equal(t, "Column duplicated.", m2.status)
}
