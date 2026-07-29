package tui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsvsheet/tsvsheet.go/internal/refresh"
	"github.com/tsvsheet/tsvsheet.go/internal/session"
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
	case "ctrl+d":
		return tea.KeyMsg{Type: tea.KeyCtrlD}
	case "ctrl+r":
		return tea.KeyMsg{Type: tea.KeyCtrlR}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
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

func TestUpdate_IgnoresUnknownMsg(t *testing.T) {
	t.Parallel()

	m := newModel(t, nil)
	next, cmd := m.Update(otherMsg{})
	assert.Equal(t, m, next)
	assert.Nil(t, cmd)
}

// tallSheet builds an n-row, single-column sheet for viewport tests.
func tallSheet(t *testing.T, n int) Model {
	t.Helper()
	s, err := session.New([]byte(strings.Repeat("r\n", n)))
	require.NoError(t, err)
	return New(s, nil, nil)
}

func TestInit(t *testing.T) {
	t.Parallel()

	assert.Nil(t, newModel(t, nil).Init())
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

// TestTUIHidesWhatTheSheetHides proves the terminal honours the declared view:
// a hidden column is not drawn, and the row numbering never renumbers — the
// gutter marks the skip instead, because those are still those rows to every
// formula in the sheet.
// TestTUIRevealShowsWithoutUnhiding proves the reveal toggle is a view of the
// file and never an edit to it: the hidden cells appear, the sheet stays clean,
// and the directive is untouched.
// TestTUIRevealSaysSoWhenThereIsNothingToReveal proves a key that would do
// nothing says why rather than appearing to fail.
// TestTUIMarksHeaderAndFrozenRows proves the other two declarations reach the
// terminal: a header row and a frozen row are drawn differently from data, which
// is all a printed grid can say about a pane that stays put while others scroll.
// TestTUIRehidingRescuesTheCursor covers the case the reveal toggle must not
// strand: the cursor sitting on a row or column that disappears when the view
// is re-hidden. It lands on the nearest rendered one instead — forward if there
// is anything below, backward when the hidden block runs to the end.
