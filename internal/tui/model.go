// Package tui is the terminal frontend: a bubbletea model over the shared
// session.Session, editing the spreadsheet grid with the same capabilities as
// the browser editor — navigate cells, edit any cell (a value or an =formula),
// recompute, and save — driven by the one engine. The model holds no language
// semantics; every mutation goes through the session.
package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/refresh"
	"github.com/tsvsheet/tsvsheet.go/internal/session"
)

// Saver persists the spreadsheet; injected so the model stays filesystem-free.
type Saver func() error

// mode is the model's current input mode.
type mode int

const (
	modeNav  mode = iota // navigating the grid
	modeEdit             // editing the selected cell's source
)

// Model is the terminal spreadsheet, a tea.Model over a session.
type Model struct {
	session          *session.Session
	save             Saver
	refresh          refresh.Next
	buffer           string
	status           string
	state            session.State
	row              int
	col              int
	viewHeight       int // terminal height in rows (0 until the first resize)
	top              int // index of the first grid row shown (vertical scroll)
	mode             mode
	isRevealing      isRevealing
	isConfirmingQuit bool
	isQuitting       bool
}

// New builds a model over a session, its saver, and an auto-refresh cadence
// (nil disables the tick), taking an initial snapshot.
func New(s *session.Session, save Saver, next refresh.Next) Model {
	return Model{session: s, save: save, refresh: next, state: s.Snapshot(), status: helpNav}
}

// tickMsg fires when the auto-refresh cadence is due.
type tickMsg time.Time

// tick schedules the next auto-refresh, or nil when the cadence is disabled or
// exhausted (an isnow schedule with no further occurrence).
func (m Model) tick() tea.Cmd {
	if m.refresh == nil {
		return nil
	}
	d := m.refresh(time.Now())
	if d <= 0 {
		return nil
	}
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// helpNav and helpEdit are the mode hints.
const (
	helpNav  = "arrows/hjkl move · enter edit · ctrl+d/ctrl+r fill · D/C duplicate row/col · ctrl+s save · R refresh imports · q quit"
	helpEdit = "type a value or =formula · enter commit · esc cancel"
)

// The key names the update loop dispatches on.
const (
	keyEnter = "enter"
	keyEsc   = "esc"
	keyCtrlC = "ctrl+c"
	keyDown  = "down"
)

// editText is an in-progress cell edit buffer.
type editText string

// Init implements tea.Model; the state is already loaded, so it only arms the
// auto-refresh tick (if any).
func (m Model) Init() tea.Cmd { return m.tick() }

// Update implements tea.Model, dispatching key messages by mode and refreshing
// volatile cells on each tick (except mid-edit, so an in-progress edit is not
// disturbed); the tick re-arms itself.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		if m.mode == modeNav {
			m.state = m.session.Recompute()
		}
		return m, m.tick()
	case tea.WindowSizeMsg:
		m.viewHeight = msg.Height
		return m.scrollToCursor(), nil
	case tea.KeyMsg:
		if m.mode == modeEdit {
			return m.keyEdit(msg)
		}
		return m.keyNav(msg)
	}
	return m, nil
}

// fillFrom copies the neighboring cell at (row, col) into the selection with
// fill semantics — ctrl+d fills from above, ctrl+r from the left, Excel's
// single-cell Ctrl+D/Ctrl+R. A selection with no such neighbor (top row, first
// column) is a no-op, as in Excel.
func (m Model) fillFrom(row, col int) Model {
	if row < 0 || col < 0 {
		return m
	}
	at := tsvsheet.Address{Row: m.row, Col: m.col}
	m.session.Fill(tsvsheet.Address{Row: row, Col: col}, tsvsheet.Span{From: at, To: at})
	m.state, m.status, m.isConfirmingQuit = m.session.Snapshot(), "Filled.", false
	return m
}

// duplicate applies a session row/column duplication at the selection and
// reports it.
func (m Model) duplicate(op func(tsvsheet.Address), status string) Model {
	op(tsvsheet.Address{Row: m.row, Col: m.col})
	m.state, m.status, m.isConfirmingQuit = m.session.Snapshot(), status, false
	return m
}

// toggleReveal shows or re-hides what the sheet's `#.hide` directives declare.
// Bound to v in navigation mode. It is session state and never an edit: the
// file is untouched, so the sheet still carries the view its author declared,
// and the cursor is pulled back onto a row that is actually rendered.
func (m Model) toggleReveal() Model {
	if !m.declaresHidden() {
		m.status = "This sheet hides nothing."
		return m
	}
	m.isRevealing = !m.isRevealing
	m.row, m.col = m.onVisibleRow(m.row), m.onVisibleCol(m.col)
	m.status = revealStatus(m.isRevealing)
	return m
}

// revealStatus names the state the toggle just moved to.
func revealStatus(isOn isRevealing) string {
	if isOn {
		return "Showing hidden rows and columns."
	}
	return "Hidden rows and columns are hidden again."
}

// refreshImports drops any cached content-typed imports and recomputes, so the
// next pass re-fetches. Bound to R in navigation mode; deliberately separate
// from the auto-refresh tick — imports never ride it (ADR 0006 §6).
func (m Model) refreshImports() Model {
	m.state, m.status, m.isConfirmingQuit = m.session.RefreshImports(), "Imports refreshed.", false
	return m
}

// doSave persists the spreadsheet, reporting the outcome.
func (m Model) doSave() Model {
	if err := m.save(); err != nil {
		m.status = err.Error()
		return m
	}
	m.session.MarkSaved()
	m.state, m.status, m.isConfirmingQuit = m.session.Snapshot(), "Saved.", false
	return m
}

// refreshedNav re-snapshots the session and returns to navigation mode.
func (m Model) refreshedNav() Model {
	m.state, m.mode, m.status, m.isConfirmingQuit = m.session.Snapshot(), modeNav, helpNav, false
	return m
}

// editBuffer applies a printable key or backspace to an edit buffer.
//
// The exhaustive exemption is deliberate and is the one case the linter's own
// rationale does not cover: tea.KeyType is bubbletea's enum, not ours, with 80+
// members that are overwhelmingly irrelevant to a text buffer. The rule exists
// so a newly added member of an OWNED enum cannot be silently absorbed by a
// default; a new key constant upstream is not a defect here, and enumerating
// every function and modifier key to say "ignore it" would bury the three that
// matter. An owned enum gets the cases written out — see proseState in
// internal/cli/man.go and structureOp in internal/serve/structure.go.
//
//nolint:exhaustive // third-party enum (tea.KeyType); default is the contract, see above
func editBuffer(buffer editText, key tea.KeyMsg) editText {
	switch key.Type {
	case tea.KeyBackspace:
		return trimLastRune(buffer)
	case tea.KeyRunes, tea.KeySpace:
		return buffer + editText(key.Runes)
	default:
		return buffer
	}
}

// trimLastRune drops the last rune of s.
func trimLastRune(s editText) editText {
	runes := []rune(s)
	if len(runes) == 0 {
		return s
	}
	return editText(runes[:len(runes)-1])
}

// clampDown decrements toward zero.
func clampDown(v cursorPos) cursorPos {
	if v <= 0 {
		return 0
	}
	return v - 1
}

// clampUp increments toward the maximum.
func clampUp(v, limit cursorPos) cursorPos {
	if v >= limit {
		return limit
	}
	return v + 1
}
