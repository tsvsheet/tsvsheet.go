// Package tui's key handling: the navigation and editing keymaps, kept apart
// from the model's own state so a binding change never touches the state
// machine.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/tsvsheet/go-tsvsheet"
)

// keyNav handles navigation-mode keys: cursor movement and commands.
func (m Model) keyNav(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if moved, handled := m.move(key.String()); handled {
		return moved, nil
	}
	return m.command(key.String())
}

// move applies a cursor movement, reporting whether the key was a movement.
func (m Model) move(key string) (Model, bool) {
	switch key {
	case "up", "k":
		m.row, m.isConfirmingQuit = int(clampDown(cursorPos(m.row))), false
	case "down", "j":
		m.row, m.isConfirmingQuit = int(clampUp(cursorPos(m.row), cursorPos(m.height()-1))), false
	case "left", "h":
		m.col, m.isConfirmingQuit = int(clampDown(cursorPos(m.col))), false
	case "right", "l":
		m.col, m.isConfirmingQuit = int(clampUp(cursorPos(m.col), cursorPos(m.width()-1))), false
	default:
		return m, false
	}
	return m.scrollToCursor(), true
}

// command handles the non-movement navigation keys.
func (m Model) command(key string) (Model, tea.Cmd) {
	switch key {
	case keyEnter, "i":
		m.mode, m.buffer, m.status, m.isConfirmingQuit = modeEdit, m.sourceAt(m.row, m.col), helpEdit, false
		return m, nil
	case "ctrl+s":
		return m.doSave(), nil
	case "ctrl+d":
		return m.fillFrom(m.row-1, m.col), nil
	case "ctrl+r":
		return m.fillFrom(m.row, m.col-1), nil
	case "D":
		return m.duplicate(m.session.DuplicateRow, "Row duplicated."), nil
	case "C":
		return m.duplicate(m.session.DuplicateCol, "Column duplicated."), nil
	case "v":
		return m.toggleReveal(), nil
	case "R":
		return m.refreshImports(), nil
	case "q", "ctrl+c", keyEsc:
		return m.quit()
	default:
		m.isConfirmingQuit = false
		return m, nil
	}
}

// quit exits, warning once when there are unsaved changes.
func (m Model) quit() (Model, tea.Cmd) {
	if m.state.IsDirty && !m.isConfirmingQuit {
		m.isConfirmingQuit, m.status = true, "Unsaved changes. Press q again to quit, or ctrl+s to save."
		return m, nil
	}
	m.isQuitting = true
	return m, tea.Quit
}

// keyEdit handles cell-edit keys.
func (m Model) keyEdit(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case keyEnter:
		return m.commit(), nil
	case keyEsc:
		return m.toNav(), nil
	default:
		m.buffer = string(editBuffer(editText(m.buffer), key))
		return m, nil
	}
}

// commit writes the buffer to the selected cell. A malformed formula keeps the
// model in edit mode (buffer preserved) and shows the error.
func (m Model) commit() Model {
	if err := m.session.SetCell(tsvsheet.Address{Row: m.row, Col: m.col}, m.buffer); err != nil {
		m.status = err.Error()
		return m
	}
	return m.refreshedNav()
}

// toNav returns to navigation mode without applying the buffer.
func (m Model) toNav() Model {
	m.mode, m.status, m.isConfirmingQuit = modeNav, helpNav, false
	return m
}
