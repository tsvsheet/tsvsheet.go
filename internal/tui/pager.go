// The pager: the terminal frontend for a windowed document (spec 016). A
// sheet over the resident cell budget is view/compute-only by ruling, so the
// pager is a deliberately separate, smaller model than the editor — scroll
// and inspect, nothing else — reading bounded viewport windows through the
// engine's WindowedSheet while the file stays on disk.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	tsvsheet "github.com/tsvsheet/go-tsvsheet"
)

// Pager is a tea.Model paging a windowed document.
type Pager struct {
	err    error
	sheet  *tsvsheet.WindowedSheet
	drift  func() error
	window tsvsheet.Grid
	opts   tsvsheet.ComputeOptions
	census tsvsheet.SheetCensus
	top    int
	height int // terminal height in rows (0 until the first resize)
	isDone bool
}

// NewPager builds a pager over a windowed sheet; opts carry the compute
// budgets and clock every viewport evaluation runs under. drift is consulted
// before each window fetch — a non-nil result renders as the error frame
// (spec 017: a file mutated in place while paged refuses honestly); nil
// disables the guard.
func NewPager(sheet *tsvsheet.WindowedSheet, opts tsvsheet.ComputeOptions, drift func() error) Pager {
	return Pager{sheet: sheet, opts: opts, census: sheet.Census(), drift: drift}
}

// Init is the bubbletea entry; the first window loads on the initial resize.
func (p Pager) Init() tea.Cmd { return nil }

// Update handles keys and resizes: navigation moves the window, q quits.
func (p Pager) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.height = msg.Height
		return p.fetched(p.top), nil
	case tea.KeyMsg:
		return p.key(msg)
	default:
		return p, nil
	}
}

// key applies one navigation key.
func (p Pager) key(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", keyCtrlC, keyEsc:
		p.isDone = true
		return p, tea.Quit
	case "up", "k":
		return p.fetched(p.top - 1), nil
	case keyDown, "j":
		return p.fetched(p.top + 1), nil
	case "pgup", "b":
		return p.fetched(p.top - p.visible()), nil
	case "pgdown", "f", " ":
		return p.fetched(p.top + p.visible()), nil
	case "home", "g":
		return p.fetched(0), nil
	case "end", "G":
		return p.fetched(p.census.Rows - p.visible()), nil
	default:
		return p, nil
	}
}

// pagerChrome is the lines the frame spends outside the grid: the status line.
const pagerChrome = 2

// visible is the number of grid rows the current terminal shows.
func (p Pager) visible() int {
	if p.height <= pagerChrome {
		return 1
	}
	return p.height - pagerChrome
}

// fetched scrolls to top (clamped to the grid) and computes that viewport,
// refusing first if the source has drifted under the pager.
func (p Pager) fetched(top int) Pager {
	p.top = min(max(top, 0), max(p.census.Rows-p.visible(), 0))
	if p.drift != nil {
		if err := p.drift(); err != nil {
			p.window, p.err = nil, err
			return p
		}
	}
	p.window, p.err = p.sheet.ComputeRows(p.top, p.visible(), p.opts)
	return p
}

// View renders the viewport with 1-based row numbers and the view-only status.
func (p Pager) View() string {
	if p.isDone {
		return ""
	}
	if p.err != nil {
		return fmt.Sprintf("error: %v\n\nq quits\n", p.err)
	}
	widths := columnWidths(p.window)
	lines := make([]string, 0, len(p.window))
	for r, row := range p.window {
		cells := make([]string, 0, len(row)+1)
		cells = append(cells, fmt.Sprintf("%8d ", p.top+r+1))
		for c, cellText := range row {
			cells = append(cells, padCell(displayText(cellText), colWidth(widths[c])))
		}
		lines = append(lines, strings.Join(cells, " "))
	}
	return strings.Join(lines, "\n") + "\n\n" + p.statusLine()
}

// statusLine names the mode and the census — why this document pages instead
// of editing.
func (p Pager) statusLine() string {
	return fmt.Sprintf(
		"view-only — %d rows × %d cols, %d cells, %d formulas (over the resident budget) · rows %d–%d · q quits",
		p.census.Rows, p.census.MaxWidth, p.census.Cells, p.census.Formulas,
		p.top+1, min(p.top+p.visible(), p.census.Rows),
	)
}

// colWidth is a column's display width in terminal cells.
type colWidth int

// padCell right-pads cellText to width display columns — measured with
// lipgloss like the editor's grid, not in bytes, so wide runes stay aligned.
func padCell(cellText displayText, width colWidth) string {
	return string(cellText) + strings.Repeat(" ", max(int(width)-lipgloss.Width(string(cellText)), 0))
}

// columnWidths sizes each column to its widest cell in the window, in display
// columns.
func columnWidths(window tsvsheet.Grid) []int {
	var widths []int
	for _, row := range window {
		for c, cellText := range row {
			for len(widths) <= c {
				widths = append(widths, 0)
			}
			widths[c] = max(widths[c], lipgloss.Width(cellText))
		}
	}
	return widths
}
