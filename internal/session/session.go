// Package session is the stateful editing model shared by every interactive
// frontend (serve, tui): one spreadsheet — a .tsvt grid of literal and formula
// cells — recomputed through the engine (the go-tsvsheet library) after each edit. It is
// the repo's one sanctioned pointer-receiver type: it wraps mutable state
// guarded for concurrent use, so a single Session backs the HTTP handlers and
// the TUI model alike.
package session

import (
	"sync"
	"time"

	"github.com/tsvsheet/go-tsvsheet"
)

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

// Session is a mutable spreadsheet. Its methods are safe for concurrent use.
type Session struct {
	loader       tsvsheet.Loader
	fetcher      tsvsheet.Fetcher
	clearImports func()
	view         tsvsheet.View
	base         tsvsheet.Path
	doc          tsvsheet.Document
	computed     tsvsheet.Grid
	diagnostics  []tsvsheet.Diagnostic
	limits       tsvsheet.Limits
	mu           sync.Mutex
	isDirty      bool
	tick         int
}

// New parses spreadsheet source and computes it eagerly, with no sheet loader —
// SHEET(...) cells resolve to #REF! — no import fetcher (every IMPORT* is
// #IMPORT!), and the engine's generous DefaultLimits. It fails on a syntax
// error; the resulting session is clean (not dirty).
func New(src []byte) (*Session, error) {
	return NewEmbeddable(src, nil, "", tsvsheet.DefaultLimits(), nil)
}

// NewEmbeddable is New with an injected sheet loader, this sheet's own path (so
// SHEET(...) cells embed other sheets resolved through loader), the resource
// limits the session enforces on every compute and edit, and the import fetcher
// content-typed IMPORT* cells fetch through (nil disables imports).
func NewEmbeddable(
	src []byte,
	loader tsvsheet.Loader,
	base tsvsheet.Path,
	limits tsvsheet.Limits,
	fetcher tsvsheet.Fetcher,
) (*Session, error) {
	// A Document, not a bare Sheet: it carries the file's physical lines, so the
	// comments, shebang and `#.` view directives a grid drops survive an edit and
	// are written back in place.
	parsed, err := tsvsheet.ParseDocument(src)
	if err != nil {
		return nil, err
	}
	s := &Session{doc: parsed, loader: loader, base: base, limits: withDefaults(limits), fetcher: fetcher}
	s.recompute()
	return s, nil
}

// withDefaults resolves the injected limits, falling the zero value (an
// unspecified Limits) back to the engine's generous DefaultLimits so a session
// never enforces a degenerate zero grid dimension.
func withDefaults(limits tsvsheet.Limits) tsvsheet.Limits {
	if limits == (tsvsheet.Limits{}) {
		return tsvsheet.DefaultLimits()
	}
	return limits
}

// recompute re-evaluates the current sheet and refreshes the read model. It uses
// the injected loader (nil disables embedding), the session limits, and samples
// the clock once.
func (s *Session) recompute() {
	sheet := s.doc.Sheet()
	s.computed = sheet.ComputeWith(s.computeOptions())
	view, viewDiags := s.doc.View()
	s.view = view
	// The directive findings first, then the cell findings: a reader meets them
	// in the order the file presents them.
	s.diagnostics = make([]tsvsheet.Diagnostic, 0, len(viewDiags))
	s.diagnostics = append(s.diagnostics, viewDiags...)
	s.diagnostics = append(s.diagnostics, tsvsheet.Check(sheet)...)
	s.tick++ // each pass advances the ordinal read by tick()/frame()
}

// computeOptions builds the compute options for this session: its loader, base
// path, and resource limits, with the clock sampled at call time.
func (s *Session) computeOptions() tsvsheet.ComputeOptions {
	return tsvsheet.ComputeOptions{
		At:      time.Now(),
		Loader:  s.loader,
		Base:    s.base,
		Limits:  s.limits,
		Fetcher: s.fetcher,
		Tick:    tsvsheet.Tick(s.tick),
	}
}

// SetCell edits one cell's source text (a literal or a formula) and recomputes.
// A malformed formula is rejected and the sheet is left unchanged (atomic); on
// success the session is marked dirty.
func (s *Session) SetCell(at tsvsheet.Address, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	updated, err := s.doc.SetCell(at, text, s.limits)
	if err != nil {
		return err
	}
	s.doc = updated
	s.isDirty = true
	s.recompute()
	return nil
}

// structuralEdit applies a whole-grid transform (a row or column insert or
// delete), recomputes, and marks the session dirty. Structural edits never
// fail: an out-of-range index is a no-op inside the engine.
func (s *Session) structuralEdit(edit func(tsvsheet.Document) tsvsheet.Document) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.doc = edit(s.doc)
	s.isDirty = true
	s.recompute()
}

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

// Recompute re-evaluates the sheet against the current clock without changing
// its source and returns the refreshed read model — for periodic refresh of
// volatile functions. It does not affect the dirty flag.
func (s *Session) Recompute() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recompute()
	return s.state()
}

// OnRefresh registers the frontend's import-cache clear, which RefreshImports
// invokes before recomputing so an explicit refresh drops cached fetches and
// re-fetches. A nil clear (or none registered) makes RefreshImports a plain
// recompute.
func (s *Session) OnRefresh(clearFn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearImports = clearFn
}

// RefreshImports drops any cached content-typed imports (via the
// frontend-injected clear) and recomputes, returning the refreshed read model.
// It is the explicit "refresh imports" action and is deliberately separate from
// the clock auto-refresh: imports never ride the isnow ticker (ADR 0006 §6). It
// is safe with no clear registered (a plain recompute) and when the sheet has no
// imports. It does not affect the dirty flag.
func (s *Session) RefreshImports() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.clearImports != nil {
		s.clearImports()
	}
	s.recompute()
	return s.state()
}

// Explain traces how the cell at addr was produced over the current sheet.
func (s *Session) Explain(addr tsvsheet.Address) (tsvsheet.Trace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return tsvsheet.Explain(s.doc.Sheet(), addr)
}

// References returns the cell at addr's precedents (the spans its formula reads)
// and dependents (the cells whose formulas read it) — the dependency edges a
// frontend highlights on selection.
func (s *Session) References(addr tsvsheet.Address) ([]tsvsheet.Span, []tsvsheet.Address) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sheet := s.doc.Sheet()
	return sheet.Precedents(addr), sheet.Dependents(addr)
}

// Embedded returns the sub-sheet a SHEET(...) cell embeds: its resolved path and
// its own computed grid, for nested rendering. ok is false when the cell is not
// a SHEET call or the reference cannot be resolved.
func (s *Session) Embedded(at tsvsheet.Address) (tsvsheet.Path, tsvsheet.Grid, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, grid, ok := s.doc.Sheet().EmbeddedGrid(at, s.computeOptions())
	return path, grid, ok
}

// MarkSaved clears the dirty flag after a frontend persists the spreadsheet.
func (s *Session) MarkSaved() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isDirty = false
}

// Source returns the spreadsheet's cell source encoded as TSV for saving.
func (s *Session) Source() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Document.Text, never a grid re-serialization: it is the one sanctioned
	// way to write a .tsvt back out, and it keeps the comment, shebang and `#.`
	// directive lines that rebuilding from the grid silently destroyed.
	return s.doc.Text()
}

// grid deep-copies a grid to a plain [][]string for a State snapshot.
func grid(g tsvsheet.Grid) [][]string {
	out := make([][]string, len(g))
	for i, row := range g {
		out[i] = append([]string(nil), row...)
	}
	return out
}
