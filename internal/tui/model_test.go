package tui

import (
	"regexp"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/refresh"
	"github.com/uplang/tsvsheet.go/internal/session"
)

func TestModel_AutoRefreshTick(t *testing.T) {
	t.Parallel()

	s, err := session.New([]byte("=now()\n"))
	require.NoError(t, err)

	// A cadence arms the tick on Init and re-arms on each tick.
	live := New(s, nil, refresh.Every(time.Millisecond))
	armed := live.Init()
	require.NotNil(t, armed)
	_, isTick := armed().(tickMsg) // running the Cmd yields a tickMsg
	assert.True(t, isTick)

	_, cmd := live.Update(tickMsg(time.Now())) // nav mode → recompute + re-arm
	require.NotNil(t, cmd)

	// In edit mode a tick skips recomputation but still re-arms.
	editing := New(s, nil, refresh.Every(time.Millisecond))
	editing.mode = modeEdit
	_, editCmd := editing.Update(tickMsg(time.Now()))
	require.NotNil(t, editCmd)

	// No cadence → no tick; an exhausted cadence (0 delay) → no tick.
	assert.Nil(t, New(s, nil, nil).Init())
	assert.Nil(t, New(s, nil, refresh.Every(0)).tick())
}

// sampleSheet is a small spreadsheet whose D column holds a formula, so the
// grid exercises both literal and formula cell styling.
var sampleSheet = []byte(
	"name\t2\t3\t=B1+C1\n" +
		"Bob\t4\t5\t=B2+C2\n",
)

func newModel(t *testing.T, save Saver) Model {
	t.Helper()
	s, err := session.New(sampleSheet)
	require.NoError(t, err)
	return New(s, save, nil)
}

// press feeds a key string to the model and returns the updated model.
func press(t *testing.T, m Model, key string) Model {
	t.Helper()
	next, _ := m.Update(keyMsg(key))
	return next.(Model)
}

// keyMsg builds a tea.KeyMsg from a key name or a single rune.
func keyMsg(key string) tea.KeyMsg {
	switch key {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}

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

func TestSave(t *testing.T) {
	t.Parallel()

	saved := false
	m := newModel(t, func() error { saved = true; return nil })
	m = press(t, m, "enter") // dirty it
	m = press(t, m, "9")
	m = press(t, m, "enter")
	require.True(t, m.state.IsDirty)

	m = press(t, m, "ctrl+s")
	assert.True(t, saved)
	assert.False(t, m.state.IsDirty)
	assert.Contains(t, m.status, "Saved")
}

func TestSave_Error(t *testing.T) {
	t.Parallel()

	m := newModel(t, func() error { return &testError{"save boom"} })
	m = press(t, m, "ctrl+s")
	assert.Contains(t, m.status, "save boom")
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

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

func TestUpdate_IgnoresNonKeyMsg(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	assert.Equal(t, m, next)
	assert.Nil(t, cmd)
}

func TestInit(t *testing.T) {
	t.Parallel()

	assert.Nil(t, newModel(t, nil).Init())
}

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

func TestEmptyGrid(t *testing.T) {
	t.Parallel()

	s, err := session.New([]byte(""))
	require.NoError(t, err)
	m := New(s, nil, nil)
	assert.Equal(t, 1, m.width())
	assert.Equal(t, 1, m.height())
	assert.NotEmpty(t, stripANSI(m.View()))
}

func TestHelpers(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "0", itoa(0))
	assert.Equal(t, "42", itoa(42))
	assert.Equal(t, "A", columnLabel(0))
	assert.Equal(t, "Z", columnLabel(25))
	assert.Equal(t, "AA", columnLabel(26))
	assert.Equal(t, "abc", clip("abc"))
	assert.Equal(t, "abcdefg…", clip("abcdefghij"))
	assert.Empty(t, cellAt([][]string{{"a"}}, 5, 0)) // row out of bounds
	assert.Empty(t, cellAt([][]string{{"a"}}, 0, 5)) // col out of bounds
	assert.Equal(t, "a", cellAt([][]string{{"a"}}, 0, 0))
}

// ansiSGR matches the ANSI SGR (color/style) escape sequences lipgloss emits.
var ansiSGR = regexp.MustCompile("\x1b\\[[0-9;]*m")

// stripANSI removes ANSI escape sequences for assertion.
func stripANSI(s string) string { return ansiSGR.ReplaceAllString(s, "") }
