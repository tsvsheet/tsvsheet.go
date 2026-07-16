package tui

// width is the number of columns to display, at least one so the cursor always
// has a column. The computed grid mirrors the source grid's shape, so either
// gives the column count.
func (m Model) width() int {
	if w := gridWidth(m.state.Computed); w > 0 {
		return w
	}
	return 1
}

// height is the number of rows to display, at least one.
func (m Model) height() int {
	if n := len(m.state.Computed); n > 0 {
		return n
	}
	return 1
}

// chromeLines is the number of non-data lines the view spends around the grid
// (title, formula bar, column header, the status bar and its top margin, and
// the trailing newline), reserved when sizing the visible row window.
const chromeLines = 6

// visibleRows is how many data rows fit the terminal: the height minus the
// chrome, at least one. Before the first resize (viewHeight 0) the whole grid
// is shown, matching the pre-viewport behavior.
func (m Model) visibleRows() int {
	if m.viewHeight <= 0 {
		return m.height()
	}
	if fit := m.viewHeight - chromeLines; fit > 0 {
		return fit
	}
	return 1
}

// visibleBounds is the half-open range of grid rows to render, [top, end).
func (m Model) visibleBounds() (int, int) {
	end := m.top + m.visibleRows()
	if end > m.height() {
		end = m.height()
	}
	return m.top, end
}

// scrollToCursor adjusts the vertical scroll so the cursor row stays on screen,
// then clamps it within the grid — the sole place `top` moves.
func (m Model) scrollToCursor() Model {
	vis := m.visibleRows()
	if m.row < m.top {
		m.top = m.row
	} else if m.row >= m.top+vis {
		m.top = m.row - vis + 1
	}
	m.top = int(clampTop(scrollOffset(m.top), scrollOffset(m.height()-vis)))
	return m
}

// scrollOffset is a vertical scroll position (rows from the top of the grid).
type scrollOffset int

// clampTop bounds a scroll offset to [0, limit]; a limit below zero (grid
// shorter than the window) pins it to the top.
func clampTop(top, limit scrollOffset) scrollOffset {
	if limit < 0 || top < 0 {
		return 0
	}
	if top > limit {
		return limit
	}
	return top
}

// gridWidth is the widest row of a grid.
func gridWidth(g [][]string) int {
	w := 0
	for _, row := range g {
		if len(row) > w {
			w = len(row)
		}
	}
	return w
}

// computedAt returns the computed value at (row, col), or empty when absent.
func (m Model) computedAt(row, col int) string {
	return cellAt(m.state.Computed, cursorPos(row), cursorPos(col))
}

// sourceAt returns the source text at (row, col), or empty when absent.
func (m Model) sourceAt(row, col int) string {
	return cellAt(m.state.Source, cursorPos(row), cursorPos(col))
}

// cellAt reads a grid cell, returning empty for any out-of-bounds position.
func cellAt(g [][]string, row, col cursorPos) string {
	r, c := int(row), int(col)
	if r < 0 || r >= len(g) || c < 0 || c >= len(g[r]) {
		return ""
	}
	return g[r][c]
}
