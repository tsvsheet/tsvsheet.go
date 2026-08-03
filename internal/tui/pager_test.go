package tui_test

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tsvsheet "github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/tui"
)

// pagerOver opens src as a windowed document (a one-cell resident budget
// forces the capability) and stands a pager over it.
func pagerOver(t *testing.T, src string) tui.Pager {
	t.Helper()
	limits := tsvsheet.DefaultLimits()
	limits.ResidentCells = 1
	_, windowed, err := tsvsheet.OpenSheet(
		tsvsheet.ByteSource{ReadAt: bytes.NewReader([]byte(src)), Size: int64(len(src))}, limits)
	require.NoError(t, err)
	require.NotNil(t, windowed)
	return tui.NewPager(windowed, tsvsheet.ComputeOptions{At: time.Now(), Limits: tsvsheet.DefaultLimits()}, nil)
}

// sized delivers the initial terminal size, loading the first window.
func sized(t *testing.T, p tui.Pager, h int) tui.Pager {
	t.Helper()
	next, _ := p.Update(tea.WindowSizeMsg{Width: 80, Height: h})
	return next.(tui.Pager)
}

// press applies one key.
func press(t *testing.T, p tui.Pager, key string) tui.Pager {
	t.Helper()
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	if len(key) > 1 {
		msg = tea.KeyMsg{Type: keyType(key)}
	}
	next, _ := p.Update(msg)
	return next.(tui.Pager)
}

// keyType maps the named keys the tests use.
func keyType(name string) tea.KeyType {
	return map[string]tea.KeyType{
		"down": tea.KeyDown, "up": tea.KeyUp,
		"pgdown": tea.KeyPgDown, "pgup": tea.KeyPgUp,
		"home": tea.KeyHome, "end": tea.KeyEnd,
		"esc": tea.KeyEsc,
	}[name]
}

// TestPagerRendersComputedWindowsAndScrollsClamped drives the pager end to
// end: the sized model shows the first computed window with 1-based row
// numbers and the view-only census status; scrolling moves the window and
// clamps at both ends; formulas show computed values.
func TestPagerRendersComputedWindowsAndScrollsClamped(t *testing.T) {
	t.Parallel()

	src := "10\t=A1*2\nr2\nr3\nr4\nr5\n"
	p := sized(t, pagerOver(t, src), 4) // 2 grid rows visible

	view := p.View()
	assert.Contains(t, view, "20", "the formula shows its computed value")
	assert.Contains(t, view, "view-only — 5 rows")
	assert.Contains(t, view, "rows 1–2")
	assert.True(t, strings.HasPrefix(view, "       1 "),
		"row numbers are 1-based and 8-wide, got %q", firstLine(view))

	p = press(t, p, "down")
	assert.Contains(t, p.View(), "rows 2–3")
	assert.True(t, strings.HasPrefix(p.View(), "       2 "),
		"the scrolled window's first row number is its true 1-based address, got %q", firstLine(p.View()))
	p = press(t, p, "end")
	assert.Contains(t, p.View(), "rows 4–5", "end clamps to the last full window")
	p = press(t, p, "down")
	assert.Contains(t, p.View(), "rows 4–5", "scrolling past the end holds")
	p = press(t, p, "home")
	p = press(t, p, "up")
	assert.Contains(t, p.View(), "rows 1–2", "scrolling above the top holds")
	p = press(t, p, "pgdown")
	assert.Contains(t, p.View(), "rows 3–4")
	p = press(t, p, "pgup")
	assert.Contains(t, p.View(), "rows 1–2")
}

// TestPagerQuitKeysEndTheProgram pins each quit chord and the blank final
// frame.
func TestPagerQuitKeysEndTheProgram(t *testing.T) {
	t.Parallel()

	for _, key := range []string{"q", "esc"} {
		p := sized(t, pagerOver(t, "a\nb\n"), 4)
		next, cmd := p.Update(keyMsgFor(key))
		require.NotNil(t, cmd, "%s must quit", key)
		assert.Empty(t, next.(tui.Pager).View(), "the final frame is blank")
	}
}

// keyMsgFor builds the message for a quit chord.
func keyMsgFor(key string) tea.Msg {
	if key == "esc" {
		return tea.KeyMsg{Type: tea.KeyEsc}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
}

// firstLine returns the frame's first line for failure messages.
func firstLine(view string) string {
	line, _, _ := strings.Cut(view, "\n")
	return line
}

// TestPagerAlignsWideRunesByDisplayWidth pins the column measure: widths are
// display columns, not bytes, so a CJK cell and its ASCII neighbours pad to
// the same visual edge.
func TestPagerAlignsWideRunesByDisplayWidth(t *testing.T) {
	t.Parallel()

	p := sized(t, pagerOver(t, "\u5bbf\u306a\tz\nabcd\tz\n"), 4)
	lines := strings.Split(p.View(), "\n")
	prefixWidth := func(line string) int {
		prefix, _, _ := strings.Cut(line, "z")
		return lipgloss.Width(prefix)
	}
	assert.Equal(t, prefixWidth(lines[0]), prefixWidth(lines[1]),
		"the second column starts at the same display column in both rows:\n%q\n%q", lines[0], lines[1])
}

// TestPagerSurfacesAComputeError pins the honest failure frame: a window that
// cannot compute shows the error with the quit hint, never a stale or partial
// grid.
func TestPagerSurfacesAComputeError(t *testing.T) {
	t.Parallel()

	limits := tsvsheet.DefaultLimits()
	limits.ResidentCells = 1
	src := "a\nb\n"
	flaky := &flakyReadAt{data: []byte(src)}
	_, windowed, err := tsvsheet.OpenSheet(tsvsheet.ByteSource{ReadAt: flaky, Size: int64(len(src))}, limits)
	require.NoError(t, err)
	require.NotNil(t, windowed)
	flaky.poisoned = true

	p := sized(t, tui.NewPager(windowed, tsvsheet.ComputeOptions{At: time.Now()}, nil), 5)
	assert.True(t, strings.HasPrefix(p.View(), "error: "), "got %q", p.View())
	assert.Contains(t, p.View(), "q quits", "the error frame still tells the user how to leave")
}

// flakyReadAt fails all reads once poisoned.
type flakyReadAt struct {
	data     []byte
	poisoned bool
}

func (f *flakyReadAt) ReadAt(p []byte, off int64) (int, error) {
	if f.poisoned {
		return 0, assert.AnError
	}
	if off >= int64(len(f.data)) {
		return 0, io.EOF
	}
	return copy(p, f.data[off:]), nil
}

// TestPagerIgnoresUnrelatedMessagesAndKeys pins the inert paths: Init
// schedules nothing (the first window loads on the initial resize), unknown
// messages and keys change nothing, and a terminal too short for any chrome
// still shows one grid row.
func TestPagerIgnoresUnrelatedMessagesAndKeys(t *testing.T) {
	t.Parallel()

	p := sized(t, pagerOver(t, "a\nb\nc\n"), 3)
	assert.Nil(t, p.Init())
	before := p.View()

	next, cmd := p.Update(struct{}{})
	assert.Nil(t, cmd)
	assert.Equal(t, before, next.(tui.Pager).View(), "an unrelated message is inert")

	next, cmd = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	assert.Nil(t, cmd)
	assert.Equal(t, before, next.(tui.Pager).View(), "an unhandled key is inert")

	tiny := sized(t, pagerOver(t, "a\nb\nc\n"), 1)
	assert.Contains(t, tiny.View(), "rows 1–1", "a too-short terminal still shows one row")
}

// TestPagerRefusesWhenTheDriftGuardTrips pins the 017 contract at the model:
// a tripped guard renders the guard's error with the quit hint and no grid —
// never a window computed over a drifted source — and a healthy guard is
// consulted without disturbing the frame.
func TestPagerRefusesWhenTheDriftGuardTrips(t *testing.T) {
	t.Parallel()

	limits := tsvsheet.DefaultLimits()
	limits.ResidentCells = 1
	src := "a\nb\nc\n"
	_, windowed, err := tsvsheet.OpenSheet(
		tsvsheet.ByteSource{ReadAt: bytes.NewReader([]byte(src)), Size: int64(len(src))}, limits)
	require.NoError(t, err)
	require.NotNil(t, windowed)

	guarded := struct{ tripped bool }{}
	drift := func() error {
		if guarded.tripped {
			return assert.AnError
		}
		return nil
	}
	p := sized(t, tui.NewPager(windowed, tsvsheet.ComputeOptions{At: time.Now()}, drift), 4)
	assert.Contains(t, p.View(), "a", "a healthy guard leaves the window alone")

	guarded.tripped = true
	p = press(t, p, "down")
	view := p.View()
	assert.True(t, strings.HasPrefix(view, "error: "), "got %q", view)
	assert.Contains(t, view, assert.AnError.Error(), "the guard's own error is the frame")
	assert.Contains(t, view, "q quits", "the refusal still tells the user how to leave")
	assert.NotContains(t, view, "view-only", "no grid or census renders over a drifted source")
}

// countingReadAt counts reads through to its backing bytes.
type countingReadAt struct {
	data  []byte
	reads int
}

func (c *countingReadAt) ReadAt(p []byte, off int64) (int, error) {
	c.reads++
	if off >= int64(len(c.data)) {
		return 0, io.EOF
	}
	return copy(p, c.data[off:]), nil
}

// TestPagerConsultsTheGuardBeforeAnyRead pins the stated ordering: a refusing
// guard means the source is not read AT ALL for that window — never computed
// first and refused after, which would pull drifted bytes into the shared
// block cache.
func TestPagerConsultsTheGuardBeforeAnyRead(t *testing.T) {
	t.Parallel()

	limits := tsvsheet.DefaultLimits()
	limits.ResidentCells = 1
	src := &countingReadAt{data: []byte("a\nb\nc\n")}
	_, windowed, err := tsvsheet.OpenSheet(tsvsheet.ByteSource{ReadAt: src, Size: int64(len(src.data))}, limits)
	require.NoError(t, err)
	require.NotNil(t, windowed)
	atOpen := src.reads

	refuse := func() error { return assert.AnError }
	p := sized(t, tui.NewPager(windowed, tsvsheet.ComputeOptions{At: time.Now()}, refuse), 4)
	assert.True(t, strings.HasPrefix(p.View(), "error: "))
	assert.Equal(t, atOpen, src.reads, "a refused window must not touch the source")
}
