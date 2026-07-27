package tui

// The view a sheet declares for itself, in the terminal. The engine resolves
// every `count(…)` and from-the-end offset against the sheet's own extent, so
// this file only decides how a terminal presents the result.
//
// Two rules match the browser editor, because a sheet must mean the same thing
// on every surface:
//
//   - Hiding never renumbers. Row 7 stays row 7 while 3–6 are hidden, so the
//     gutter skips numbers and the discontinuity is marked rather than silently
//     closing up.
//   - Revealing never writes. It is session state; the file is untouched, so a
//     sheet still carries the view its author declared.

// axisPos is a 1-based row or column position, the numbering a directive writes
// and the gutter shows — not the 0-based index the grid is stored in.
type axisPos int

// isRevealing reports whether the terminal is currently showing what the sheet
// hides.
type isRevealing bool

// hiddenRow reports whether a 0-based grid row is hidden and not being revealed.
func (m Model) hiddenRow(row int) bool {
	return !bool(m.isRevealing) && m.state.View.HiddenRows[int(axisPos(row+1))]
}

// hiddenCol reports whether a 0-based grid column is hidden and not being
// revealed.
func (m Model) hiddenCol(col int) bool {
	return !bool(m.isRevealing) && m.state.View.HiddenCols[int(axisPos(col+1))]
}

// declaresHidden reports whether the sheet hides anything at all, which is what
// the reveal toggle is for: a key that does nothing is worse than no key.
func (m Model) declaresHidden() bool {
	return len(m.state.View.HiddenRows) > 0 || len(m.state.View.HiddenCols) > 0
}

// headerRow reports whether a 0-based grid row is declared as a header.
func (m Model) headerRow(row int) bool {
	return m.state.View.HeaderRows[int(axisPos(row+1))]
}

// frozenRow reports whether a 0-based grid row is pinned by a freeze directive.
func (m Model) frozenRow(row int) bool {
	return m.state.View.FreezeRows[int(axisPos(row+1))]
}

// visibleCols lists the 0-based columns to render, in order.
func (m Model) visibleCols() []int {
	cols := make([]int, 0, m.width())
	for c := 0; c < m.width(); c++ {
		if !m.hiddenCol(c) {
			cols = append(cols, c)
		}
	}
	return cols
}

// rowLabel is the gutter text for a row: its own 1-based number, prefixed with
// a marker when rows above it were skipped, so a reader sees that the numbering
// jumped on purpose.
func (m Model) rowLabel(row int) displayText {
	label := displayText(itoa(displayInt(row + 1)))
	if m.skippedAbove(row) {
		return "⋯" + label
	}
	return label
}

// onVisibleRow pulls a cursor row onto one that is actually rendered, so
// re-hiding never strands the cursor on a row that just disappeared.
func (m Model) onVisibleRow(row int) int {
	for at := row; at < m.height(); at++ {
		if !m.hiddenRow(at) {
			return at
		}
	}
	for at := row; at >= 0; at-- {
		if !m.hiddenRow(at) {
			return at
		}
	}
	return row
}

// onVisibleCol is onVisibleRow for the column axis.
func (m Model) onVisibleCol(col int) int {
	cols := m.visibleCols()
	for _, c := range cols {
		if c >= col {
			return c
		}
	}
	if len(cols) > 0 {
		return cols[len(cols)-1]
	}
	return col
}

// skippedAbove reports whether the row immediately above this one is hidden, so
// the gutter can mark the discontinuity.
func (m Model) skippedAbove(row int) bool {
	return row > 0 && m.hiddenRow(row-1)
}
