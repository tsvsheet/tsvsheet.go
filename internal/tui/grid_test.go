package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsvsheet/tsvsheet.go/internal/session"
)

func TestViewport_ScrollsToKeepCursorVisible(t *testing.T) {
	t.Parallel()

	m := tallSheet(t, 30)
	// Before any resize the whole grid is shown (viewHeight 0 → all rows).
	assert.Equal(t, m.height(), m.visibleRows())

	// A short window: height 10 → 10-6 chrome = 4 visible data rows.
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	m = next.(Model)
	assert.Nil(t, cmd)
	assert.Equal(t, 4, m.visibleRows())
	assert.Equal(t, 0, m.top) // cursor at top → no scroll

	// Move down past the window; the view scrolls to follow (down branch).
	for i := 0; i < 20; i++ {
		m = press(t, m, "down")
	}
	assert.Equal(t, 20, m.row)
	assert.Equal(t, 17, m.top) // row - visible + 1
	top, end := m.visibleBounds()
	assert.Equal(t, 17, top)
	assert.Equal(t, 21, end)

	// The grid renders only the visible slice: header + 4 rows = 5 lines.
	assert.Equal(t, 5, strings.Count(stripANSI(m.grid()), "\n")+1)

	// Move back up; the view scrolls up (up branch) and returns to the top.
	for i := 0; i < 25; i++ {
		m = press(t, m, "up")
	}
	assert.Equal(t, 0, m.row)
	assert.Equal(t, 0, m.top)
}

func TestViewport_ShortSheetAndTinyWindow(t *testing.T) {
	t.Parallel()

	// A window smaller than the chrome still shows one data row.
	tiny, _ := tallSheet(t, 30).Update(tea.WindowSizeMsg{Width: 40, Height: 3})
	assert.Equal(t, 1, tiny.(Model).visibleRows())

	// A short sheet in a tall window: bounds clamp to the grid height.
	short := newModel(t, nil) // 2 rows
	sized, _ := short.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
	top, end := sized.(Model).visibleBounds()
	assert.Equal(t, 0, top)
	assert.Equal(t, sized.(Model).height(), end) // end clamped to 2
}

func TestClampTop(t *testing.T) {
	t.Parallel()

	assert.Equal(t, scrollOffset(0), clampTop(5, -1))   // grid shorter than window → pin top
	assert.Equal(t, scrollOffset(0), clampTop(-3, 10))  // negative offset → top
	assert.Equal(t, scrollOffset(10), clampTop(15, 10)) // beyond the last page → last page
	assert.Equal(t, scrollOffset(7), clampTop(7, 10))   // within range → unchanged
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

// TestWidth_IsAtLeastOneSoTheCursorAlwaysHasAColumn pins the floor the doc
// names. A zero width would leave the cursor with nowhere to be, and every
// clamp that bounds col by width-1 would produce -1.
func TestWidth_IsAtLeastOneSoTheCursorAlwaysHasAColumn(t *testing.T) {
	t.Parallel()

	empty, err := session.New([]byte(""))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, New(empty, nil, nil).width(), 1, "even an empty sheet has a column")

	wide, err := session.New([]byte("a\tb\tc\n"))
	require.NoError(t, err)
	assert.Equal(t, 3, New(wide, nil, nil).width())
}
