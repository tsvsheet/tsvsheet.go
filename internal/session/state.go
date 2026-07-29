// Package session's observable state: the immutable snapshot a frontend renders
// from — values, diagnostics, the declared view — and the volatility questions
// a refreshing frontend asks before scheduling another pass.
package session

import "github.com/tsvsheet/go-tsvsheet"

// State is the complete read model a frontend renders: the computed value grid,
// the cell source texts (literals and "=formulas") for editing, static
// diagnostics, and the dirty flag. It is a value snapshot; mutating it never
// affects the Session.
type State struct {
	View        tsvsheet.View         `json:"view"`
	Computed    [][]string            `json:"computed"`
	Source      [][]string            `json:"source"`
	Diagnostics []tsvsheet.Diagnostic `json:"diagnostics"`
	IsDirty     bool                  `json:"dirty"`
}

// Snapshot returns a deep-copied read model safe for the caller to hold and
// mutate.
func (s *Session) Snapshot() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state()
}

// state builds the read model; the caller holds s.mu.
func (s *Session) state() State {
	return State{
		Computed:    grid(s.computed),
		Source:      grid(s.doc.Sheet().Source()),
		View:        s.view,
		Diagnostics: append([]tsvsheet.Diagnostic(nil), s.diagnostics...),
		IsDirty:     s.isDirty,
	}
}

// IsVolatile reports whether the sheet wraps any expression in volatile(…), so a
// frontend can enable periodic recomputation.
func (s *Session) IsVolatile() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.doc.Sheet().IsVolatile()
}

// VolatileSchedules returns the refresh-cadence spec of every volatile(…) cell
// (empty for one with no schedule of its own), which a frontend unions into a
// single auto-refresh cadence.
func (s *Session) VolatileSchedules() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.doc.Sheet().VolatileSchedules()
}
