package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tsvsheet/go-tsvsheet"
)

// cellWidth is the fixed display width of a grid cell.
const cellWidth = 8

// cursorPos is a 0-based grid coordinate (row or column).
type cursorPos int

// displayText is a rendered fragment; displayInt a value shown in the grid.
type (
	displayText string
	displayInt  int
)

// styles for the terminal grid, tuned for a monospace "ledger" look matching
// the web UI: muted headers, tinted formula cells, flagged error values.
var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	headStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Align(lipgloss.Center).Width(cellWidth)
	dataStyle     = lipgloss.NewStyle().Align(lipgloss.Right).Width(cellWidth)
	formulaStyle  = lipgloss.NewStyle().Align(lipgloss.Right).Width(cellWidth).Foreground(lipgloss.Color("179"))
	errStyle      = lipgloss.NewStyle().Align(lipgloss.Right).Width(cellWidth).Foreground(lipgloss.Color("203"))
	cursorStyle   = lipgloss.NewStyle().Align(lipgloss.Right).Width(cellWidth).Reverse(true)
	headerRowSty  = lipgloss.NewStyle().Align(lipgloss.Right).Width(cellWidth).Bold(true)
	frozenSty     = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Align(lipgloss.Center).Width(cellWidth)
	hiddenSty     = lipgloss.NewStyle().Align(lipgloss.Right).Width(cellWidth).Faint(true)
	statusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).MarginTop(1)
	dirtyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("179"))
	formulaBarSty = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
)

// View implements tea.Model.
func (m Model) View() string {
	if m.isQuitting {
		return ""
	}
	sections := []string{m.titleBar(), m.formulaBar(), m.grid(), m.statusBar()}
	return strings.Join(sections, "\n") + "\n"
}

// titleBar renders the header line with the dirty indicator.
func (m Model) titleBar() string {
	state := "saved"
	if m.state.IsDirty {
		state = dirtyStyle.Render("● unsaved")
	}
	return titleStyle.Render("tsvsheet") + "  " + state
}

// formulaBar shows the selected cell's address and its source (a literal or an
// =formula), reflecting the in-progress edit buffer while editing.
func (m Model) formulaBar() string {
	addr := tsvsheet.Address{Row: m.row, Col: m.col}.String()
	source := m.sourceAt(m.row, m.col)
	if m.mode == modeEdit {
		source = m.buffer + "▏"
	}
	return formulaBarSty.Render(addr+": ") + source
}

// grid renders the visible slice of the computed sheet with column letters, row
// numbers, and the cursor highlight, scrolled vertically to keep the cursor on
// screen.
func (m Model) grid() string {
	rows := []string{m.columnHeadings()}
	top, end := m.visibleBounds()
	for r := top; r < end; r++ {
		if m.hiddenRow(r) {
			continue
		}
		rows = append(rows, m.gridRow(r))
	}
	return strings.Join(rows, "\n")
}

// columnHeadings renders the column-letter row, skipping hidden columns while
// keeping every other column's own letter — hiding column C leaves B beside D,
// because they are still B and D to every formula in the sheet.
func (m Model) columnHeadings() string {
	visible := m.visibleCols()
	cells := make([]string, 0, len(visible)+1)
	cells = append(cells, headStyle.Render(""))
	for _, c := range visible {
		cells = append(cells, headStyle.Render(columnLabel(cursorPos(c))))
	}
	return strings.Join(cells, " ")
}

// gridRow renders one data row with its row number, which never renumbers: the
// gutter marks a skip rather than closing it up.
func (m Model) gridRow(row int) string {
	visible := m.visibleCols()
	cells := make([]string, 0, len(visible)+1)
	cells = append(cells, m.gutterStyle(row).Render(string(m.rowLabel(row))))
	for _, c := range visible {
		cells = append(cells, m.renderCell(row, c))
	}
	return strings.Join(cells, " ")
}

// gutterStyle tints the row number of a frozen row, the one declaration a
// terminal cannot express by position: a pane that stays put while the rest
// scrolls has no counterpart in a printed grid.
func (m Model) gutterStyle(row int) lipgloss.Style {
	if m.frozenRow(row) {
		return frozenSty
	}
	return headStyle
}

// renderCell styles one grid cell by its kind and cursor state.
func (m Model) renderCell(row, col int) string {
	value := m.computedAt(row, col)
	return m.cellStyle(row, col, value).Render(clip(displayText(value)))
}

// cellStyle selects the style for a cell: cursor, error, formula, or literal.
func (m Model) cellStyle(row, col int, value string) lipgloss.Style {
	if row == m.row && col == m.col {
		return cursorStyle
	}
	if isErrorValue(displayText(value)) {
		return errStyle
	}
	if strings.HasPrefix(m.sourceAt(row, col), "=") {
		return formulaStyle
	}
	if m.state.View.HiddenRows[row+1] || m.state.View.HiddenCols[col+1] {
		return hiddenSty // only reachable while revealing
	}
	if m.headerRow(row) {
		return headerRowSty
	}
	return dataStyle
}

// statusBar renders the mode hint and diagnostics count.
func (m Model) statusBar() string {
	line := m.status
	if n := len(m.state.Diagnostics); n > 0 {
		line = errStyleInline(displayText(itoa(displayInt(n))+" diagnostic(s)")) + "  " + line
	}
	return statusStyle.Render(line)
}

// errStyleInline renders an inline error-colored fragment.
func errStyleInline(s displayText) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render(string(s))
}

// clip trims a value to the cell width so the grid stays aligned.
func clip(s displayText) string {
	runes := []rune(s)
	if len(runes) <= cellWidth {
		return string(s)
	}
	return string(runes[:cellWidth-1]) + "…"
}

// isErrorValue reports whether a cell value is a spreadsheet error value.
func isErrorValue(value displayText) bool {
	switch tsvsheet.ErrorValue(value) {
	case tsvsheet.ErrRef, tsvsheet.ErrValue, tsvsheet.ErrName, tsvsheet.ErrDiv, tsvsheet.ErrCirc:
		return true
	default:
		return false
	}
}

// columnLabel converts a 0-based column index to spreadsheet letters.
func columnLabel(index cursorPos) string {
	label := ""
	for n := index + 1; n > 0; n = (n - 1) / 26 {
		label = string(rune('A'+(n-1)%26)) + label
	}
	return label
}

// itoa renders a non-negative int without importing strconv at each call site.
func itoa(n displayInt) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for ; n > 0; n /= 10 {
		digits = string(rune('0'+n%10)) + digits
	}
	return digits
}
