package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/uplang/tsvsheet.go/internal/sheet"
)

// cellWidth is the fixed display width of a grid cell.
const cellWidth = 8

// styles for the terminal grid, tuned for a monospace "ledger" look matching
// the web UI: muted headers, tinted computed cells, flagged error values.
var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	headStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Align(lipgloss.Center).Width(cellWidth)
	dataStyle     = lipgloss.NewStyle().Align(lipgloss.Right).Width(cellWidth)
	computedStyle = lipgloss.NewStyle().Align(lipgloss.Right).Width(cellWidth).Foreground(lipgloss.Color("179"))
	errStyle      = lipgloss.NewStyle().Align(lipgloss.Right).Width(cellWidth).Foreground(lipgloss.Color("203"))
	cursorStyle   = lipgloss.NewStyle().Align(lipgloss.Right).Width(cellWidth).Reverse(true)
	statusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).MarginTop(1)
	dirtyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("179"))
	paneStyle     = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1).MarginTop(1)
)

// View implements tea.Model.
func (m Model) View() string {
	if m.quitting {
		return ""
	}
	sections := []string{m.titleBar(), m.grid()}
	if m.mode == modeTemplate {
		sections = append(sections, m.templatePane())
	}
	sections = append(sections, m.statusBar())
	return strings.Join(sections, "\n") + "\n"
}

// titleBar renders the header line with the dirty indicator.
func (m Model) titleBar() string {
	state := "saved"
	if m.state.Dirty {
		state = dirtyStyle.Render("● unsaved")
	}
	return titleStyle.Render("tsvsheet") + "  " + state
}

// grid renders the computed sheet with column letters, row numbers, and the
// cursor highlight.
func (m Model) grid() string {
	rows := []string{m.headerRow()}
	for r := 0; r < m.height(); r++ {
		rows = append(rows, m.gridRow(r))
	}
	return strings.Join(rows, "\n")
}

// headerRow renders the column-letter header.
func (m Model) headerRow() string {
	cells := []string{headStyle.Render("")}
	for c := 0; c < m.width(); c++ {
		cells = append(cells, headStyle.Render(columnLabel(c)))
	}
	return strings.Join(cells, " ")
}

// gridRow renders one data row with its row number.
func (m Model) gridRow(row int) string {
	cells := []string{headStyle.Render(itoa(row + 1))}
	for c := 0; c < m.width(); c++ {
		cells = append(cells, m.renderCell(row, c))
	}
	return strings.Join(cells, " ")
}

// renderCell styles one grid cell by its kind and cursor state.
func (m Model) renderCell(row, col int) string {
	value := m.computedValue(row, col)
	return m.cellStyle(row, col, value).Render(clip(value))
}

// cellStyle selects the style for a cell: cursor, error, computed, or data.
func (m Model) cellStyle(row, col int, value string) lipgloss.Style {
	if row == m.row && col == m.col {
		return cursorStyle
	}
	if isErrorValue(value) {
		return errStyle
	}
	if !m.editable(row, col) {
		return computedStyle
	}
	return dataStyle
}

// statusBar renders the mode hint, edit buffer, and diagnostics count.
func (m Model) statusBar() string {
	line := m.status
	if m.mode == modeCell {
		line = "» " + m.buffer + "▏   " + m.status
	}
	if n := len(m.state.Diagnostics); n > 0 {
		line = errStyleInline(itoa(n)+" diagnostic(s)") + "  " + line
	}
	return statusStyle.Render(line)
}

// templatePane renders the template edit buffer.
func (m Model) templatePane() string {
	return paneStyle.Render(m.buffer + "▏")
}

// errStyleInline renders an inline error-colored fragment.
func errStyleInline(s string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render(s)
}

// clip trims a value to the cell width so the grid stays aligned.
func clip(s string) string {
	runes := []rune(s)
	if len(runes) <= cellWidth {
		return s
	}
	return string(runes[:cellWidth-1]) + "…"
}

// isErrorValue reports whether a cell value is a spreadsheet error value.
func isErrorValue(value string) bool {
	switch sheet.ErrorValue(value) {
	case sheet.ErrRef, sheet.ErrValue, sheet.ErrName, sheet.ErrDiv:
		return true
	default:
		return false
	}
}

// columnLabel converts a 0-based column index to spreadsheet letters.
func columnLabel(index int) string {
	label := ""
	for n := index + 1; n > 0; n = (n - 1) / 26 {
		label = string(rune('A'+(n-1)%26)) + label
	}
	return label
}

// itoa renders a non-negative int without importing strconv at each call site.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for ; n > 0; n /= 10 {
		digits = string(rune('0'+n%10)) + digits
	}
	return digits
}
