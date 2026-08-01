// Package session's structural edits: the row and column operations that change
// the grid's shape. Each goes through one seam so a directive-aware document
// edit and the recompute that follows stay together.
package session

import "github.com/tsvsheet/go-tsvsheet"

// InsertRow inserts a blank row before at.Row, shifting references down.
func (s *Session) InsertRow(at tsvsheet.Address) {
	s.structuralEdit(func(d tsvsheet.Document) tsvsheet.Document { return d.InsertRow(at) })
}

// DeleteRow removes row at.Row, turning references to it into #REF!.
func (s *Session) DeleteRow(at tsvsheet.Address) {
	s.structuralEdit(func(d tsvsheet.Document) tsvsheet.Document { return d.DeleteRow(at) })
}

// InsertCol inserts a blank column before at.Col, shifting references right.
func (s *Session) InsertCol(at tsvsheet.Address) {
	s.structuralEdit(func(d tsvsheet.Document) tsvsheet.Document { return d.InsertCol(at) })
}

// DeleteCol removes column at.Col, turning references to it into #REF!.
func (s *Session) DeleteCol(at tsvsheet.Address) {
	s.structuralEdit(func(d tsvsheet.Document) tsvsheet.Document { return d.DeleteCol(at) })
}

// Fill copies the cell at from across the to span with fill semantics: each
// unpinned reference shifts by the target's offset, `$`-pinned coordinates
// hold (Sheet.Fill).
func (s *Session) Fill(from tsvsheet.Address, to tsvsheet.Span) {
	s.structuralEdit(func(d tsvsheet.Document) tsvsheet.Document { return d.Fill(from, to) })
}

// DuplicateRow duplicates row at.Row below itself: the duplicate's references
// rebase one row down and the rest of the grid shifts as InsertRow shifts it.
func (s *Session) DuplicateRow(at tsvsheet.Address) {
	s.structuralEdit(func(d tsvsheet.Document) tsvsheet.Document { return d.DuplicateRow(at) })
}

// DuplicateCol duplicates column at.Col to its right: the duplicate's
// references rebase one column right and the rest of the grid shifts as
// InsertCol shifts it.
func (s *Session) DuplicateCol(at tsvsheet.Address) {
	s.structuralEdit(func(d tsvsheet.Document) tsvsheet.Document { return d.DuplicateCol(at) })
}

// structuralEdit applies a whole-grid transform (a row or column insert or
// delete), recomputes, and marks the session dirty. Structural edits do not
// fail: an out-of-range index is a no-op inside the engine.
func (s *Session) structuralEdit(edit func(tsvsheet.Document) tsvsheet.Document) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.doc = edit(s.doc)
	s.isDirty = true
	s.recompute()
}
