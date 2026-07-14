package tui

// width is the number of columns to display: the widest of the data and
// computed grids, at least one so the cursor always has a column.
func (m Model) width() int {
	w := dataWidth(m.state.Data)
	for _, row := range m.state.Computed {
		if len(row) > w {
			w = len(row)
		}
	}
	if w == 0 {
		return 1
	}
	return w
}

// height is the number of rows to display, at least one.
func (m Model) height() int {
	if n := len(m.state.Computed); n > 0 {
		return n
	}
	return 1
}

// dataWidth is the widest row of a grid.
func dataWidth(g [][]string) int {
	w := 0
	for _, row := range g {
		if len(row) > w {
			w = len(row)
		}
	}
	return w
}

// editable reports whether (row, col) is a raw data cell (not a computed-only
// cell), and therefore editable.
func (m Model) editable(row, col int) bool {
	return row < len(m.state.Data) && col < dataWidth(m.state.Data)
}

// dataValue returns the raw data value at (row, col), or empty when absent.
func (m Model) dataValue(row, col int) string {
	return cellAt(m.state.Data, row, col)
}

// computedValue returns the computed value at (row, col), or empty when absent.
func (m Model) computedValue(row, col int) string {
	return cellAt(m.state.Computed, row, col)
}

// cellAt reads a grid cell, returning empty for any out-of-bounds position.
func cellAt(g [][]string, row, col int) string {
	if row < 0 || row >= len(g) || col < 0 || col >= len(g[row]) {
		return ""
	}
	return g[row][col]
}
