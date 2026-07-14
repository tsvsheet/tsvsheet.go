package tui

import (
	"regexp"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/session"
	"github.com/uplang/tsvsheet.go/internal/sheet"
)

var (
	sampleData     = sheet.Grid{{"1", "2", "3", "4"}, {"2", "3", "4", "5"}, {"3", "4", "5", "6"}}
	sampleTemplate = []byte("=body\nE=C + D\n")
)

func newModel(t *testing.T, save Saver) Model {
	t.Helper()
	s, err := session.New(sampleTemplate, sampleData)
	require.NoError(t, err)
	return New(s, save)
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
	case "ctrl+d":
		return tea.KeyMsg{Type: tea.KeyCtrlD}
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

func TestEditDataCell(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	m = press(t, m, "enter") // edit A1
	assert.Equal(t, modeCell, m.mode)

	m = press(t, m, "backspace") // clear "1"
	m = press(t, m, "9")
	m = press(t, m, "enter") // commit
	assert.Equal(t, modeNav, m.mode)

	state := m.state
	assert.Equal(t, "9", state.Data[0][0])
	assert.True(t, state.Dirty)
}

func TestEditDataCell_Space(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	m = press(t, m, "enter")
	m = press(t, m, "space")
	assert.Contains(t, m.buffer, " ")
}

func TestEditDataCell_Cancel(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	m = press(t, m, "enter")
	m = press(t, m, "5")
	m = press(t, m, "esc") // cancel
	assert.Equal(t, modeNav, m.mode)
	assert.Equal(t, "1", m.state.Data[0][0]) // unchanged
}

func TestEdit_ComputedCellRejected(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	m = press(t, m, "right") // B
	m = press(t, m, "right") // C
	m = press(t, m, "right") // D
	m = press(t, m, "right") // E (computed)
	m = press(t, m, "enter")
	assert.Equal(t, modeNav, m.mode)
	assert.Contains(t, m.status, "computed")
}

func TestEditTemplate(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	m = press(t, m, "t")
	assert.Equal(t, modeTemplate, m.mode)
	assert.Equal(t, string(sampleTemplate), m.buffer)

	// Append a new final line and apply.
	for _, r := range "\n=final" {
		if r == '\n' {
			m = press(t, m, "enter")
		} else {
			m = press(t, m, string(r))
		}
	}
	m = press(t, m, "ctrl+d") // apply
	assert.Equal(t, modeNav, m.mode)
}

func TestEditTemplate_SyntaxErrorStays(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	m = press(t, m, "t")
	// Replace buffer with invalid syntax.
	m.buffer = "=sum("
	m = press(t, m, "ctrl+d")
	assert.Equal(t, modeTemplate, m.mode) // stays so the buffer is not lost
	assert.NotEmpty(t, m.status)
}

func TestEditTemplate_Cancel(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	m = press(t, m, "t")
	m = press(t, m, "x")
	m = press(t, m, "esc")
	assert.Equal(t, modeNav, m.mode)
	assert.Equal(t, string(sampleTemplate), m.state.Template) // unchanged
}

func TestEditTemplate_Backspace(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	m = press(t, m, "t")
	before := m.buffer
	m = press(t, m, "backspace")
	assert.Equal(t, before[:len(before)-1], m.buffer)
}

func TestSave(t *testing.T) {
	t.Parallel()

	saved := false
	m := newModel(t, func() error { saved = true; return nil })
	m = press(t, m, "enter") // dirty it
	m = press(t, m, "9")
	m = press(t, m, "enter")
	require.True(t, m.state.Dirty)

	m = press(t, m, "ctrl+s")
	assert.True(t, saved)
	assert.False(t, m.state.Dirty)
	assert.Contains(t, m.status, "Saved")
}

func TestSave_Error(t *testing.T) {
	t.Parallel()

	m := newModel(t, func() error { return assertErr })
	m = press(t, m, "ctrl+s")
	assert.Contains(t, m.status, "save boom")
}

var assertErr = &testError{"save boom"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

func TestQuit_Clean(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	next, cmd := m.Update(keyMsg("q"))
	assert.True(t, next.(Model).quitting)
	assert.NotNil(t, cmd) // tea.Quit
}

func TestQuit_DirtyWarnsThenQuits(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	m = press(t, m, "enter")
	m = press(t, m, "9")
	m = press(t, m, "enter") // dirty

	m = press(t, m, "q") // first q → warn
	assert.False(t, m.quitting)
	assert.True(t, m.confirmQuit)
	assert.Contains(t, m.status, "Unsaved")

	next, cmd := m.Update(keyMsg("q")) // second q → quit
	assert.True(t, next.(Model).quitting)
	assert.NotNil(t, cmd)
}

func TestQuit_DirtyThenOtherKeyResets(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	m = press(t, m, "enter")
	m = press(t, m, "9")
	m = press(t, m, "enter")
	m = press(t, m, "q")    // warn
	m = press(t, m, "down") // movement resets confirm
	assert.False(t, m.confirmQuit)

	m = newModel(t, nil)
	m = press(t, m, "q") // clean, quits — restart for the command-reset path
	m = newModel(t, nil)
	m = press(t, m, "enter")
	m = press(t, m, "9")
	m = press(t, m, "enter")
	m = press(t, m, "q")      // warn
	m = press(t, m, "ctrl+c") // ctrl+c after warn → quits
	assert.True(t, m.quitting)
}

func TestUnhandledKeyResetsConfirm(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	m = press(t, m, "z") // unknown nav key
	assert.False(t, m.confirmQuit)
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

func TestView_Modes(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	assert.Contains(t, stripANSI(m.View()), "tsvsheet")

	editing := press(t, m, "enter")
	assert.Contains(t, stripANSI(editing.View()), "»") // cell buffer shown

	tmpl := press(t, m, "t")
	assert.Contains(t, stripANSI(tmpl.View()), "=body") // template pane

	quit := press(t, m, "q")
	assert.Empty(t, quit.View())
}

func TestView_DirtyAndDiagnostics(t *testing.T) {
	t.Parallel()

	s, err := session.New([]byte("=body\nZ=bogus(A)\n"), sampleData) // unknown func → diagnostic
	require.NoError(t, err)
	m := New(s, nil)
	view := stripANSI(m.View())
	assert.Contains(t, view, "diagnostic")

	m = press(t, m, "enter")
	m = press(t, m, "9")
	m = press(t, m, "enter")
	assert.Contains(t, stripANSI(m.View()), "unsaved")
}

func TestView_ErrorAndLongValues(t *testing.T) {
	t.Parallel()

	// A formula producing #REF! and a long literal exercise error and clip
	// styling.
	s, err := session.New([]byte("=body\nE=C9\nF=1234567890\n"), sampleData)
	require.NoError(t, err)
	view := stripANSI(New(s, nil).View())
	assert.Contains(t, view, "#REF!")
	assert.Contains(t, view, "…") // clipped long value
}

func TestEmptyGrid(t *testing.T) {
	t.Parallel()

	s, err := session.New([]byte("=body\n"), sheet.Grid{})
	require.NoError(t, err)
	m := New(s, nil)
	assert.Equal(t, 1, m.width())
	assert.Equal(t, 1, m.height())
	assert.NotEmpty(t, stripANSI(m.View()))
}

func TestEditBuffer_UnhandledAndEmptyBackspace(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	m = press(t, m, "enter")     // edit A1, buffer "1"
	m = press(t, m, "backspace") // ""
	m = press(t, m, "backspace") // backspace on empty → no-op
	assert.Empty(t, m.buffer)
	m = press(t, m, "left") // unhandled key in cell mode → buffer unchanged
	assert.Empty(t, m.buffer)
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
