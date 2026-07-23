// Package session is the stateful editing model shared by every interactive
// frontend (serve, tui): one spreadsheet — a .tsvt grid of literal and formula
// cells — recomputed through the engine (the go-tsvsheet library) after each edit. It is
// the repo's one sanctioned pointer-receiver type: it wraps mutable state
// guarded for concurrent use, so a single Session backs the HTTP handlers and
// the TUI model alike.
package session

import (
	"bytes"
	"sync"
	"time"

	"github.com/tsvsheet/go-tsvsheet"
)

// State is the complete read model a frontend renders: the computed value grid,
// the cell source texts (literals and "=formulas") for editing, static
// diagnostics, and the dirty flag. It is a value snapshot; mutating it never
// affects the Session.
type State struct {
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
	base         tsvsheet.Path
	sheet        tsvsheet.Sheet
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
	parsed, err := tsvsheet.Parse(src)
	if err != nil {
		return nil, err
	}
	s := &Session{sheet: parsed, loader: loader, base: base, limits: withDefaults(limits), fetcher: fetcher}
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
	s.computed = s.sheet.ComputeWith(s.computeOptions())
	s.diagnostics = tsvsheet.Check(s.sheet)
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
	updated, err := s.sheet.Set(at, text, s.limits)
	if err != nil {
		return err
	}
	s.sheet = updated
	s.isDirty = true
	s.recompute()
	return nil
}

// structuralEdit applies a whole-grid transform (a row or column insert or
// delete), recomputes, and marks the session dirty. Structural edits never
// fail: an out-of-range index is a no-op inside the engine.
func (s *Session) structuralEdit(edit func(tsvsheet.Sheet) tsvsheet.Sheet) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sheet = edit(s.sheet)
	s.isDirty = true
	s.recompute()
}

// InsertRow inserts a blank row before at.Row, shifting references down.
func (s *Session) InsertRow(at tsvsheet.Address) {
	s.structuralEdit(func(sh tsvsheet.Sheet) tsvsheet.Sheet { return sh.InsertRow(at) })
}

// DeleteRow removes row at.Row, turning references to it into #REF!.
func (s *Session) DeleteRow(at tsvsheet.Address) {
	s.structuralEdit(func(sh tsvsheet.Sheet) tsvsheet.Sheet { return sh.DeleteRow(at) })
}

// InsertCol inserts a blank column before at.Col, shifting references right.
func (s *Session) InsertCol(at tsvsheet.Address) {
	s.structuralEdit(func(sh tsvsheet.Sheet) tsvsheet.Sheet { return sh.InsertCol(at) })
}

// DeleteCol removes column at.Col, turning references to it into #REF!.
func (s *Session) DeleteCol(at tsvsheet.Address) {
	s.structuralEdit(func(sh tsvsheet.Sheet) tsvsheet.Sheet { return sh.DeleteCol(at) })
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
		Source:      grid(s.sheet.Source()),
		Diagnostics: append([]tsvsheet.Diagnostic(nil), s.diagnostics...),
		IsDirty:     s.isDirty,
	}
}

// IsVolatile reports whether the sheet wraps any expression in volatile(…), so a
// frontend can enable periodic recomputation.
func (s *Session) IsVolatile() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sheet.IsVolatile()
}

// VolatileSchedules returns the refresh-cadence spec of every volatile(…) cell
// (empty for one with no schedule of its own), which a frontend unions into a
// single auto-refresh cadence.
func (s *Session) VolatileSchedules() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sheet.VolatileSchedules()
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
	return tsvsheet.Explain(s.sheet, addr)
}

// References returns the cell at addr's precedents (the spans its formula reads)
// and dependents (the cells whose formulas read it) — the dependency edges a
// frontend highlights on selection.
func (s *Session) References(addr tsvsheet.Address) ([]tsvsheet.Span, []tsvsheet.Address) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sheet.Precedents(addr), s.sheet.Dependents(addr)
}

// Embedded returns the sub-sheet a SHEET(...) cell embeds: its resolved path and
// its own computed grid, for nested rendering. ok is false when the cell is not
// a SHEET call or the reference cannot be resolved.
func (s *Session) Embedded(at tsvsheet.Address) (tsvsheet.Path, tsvsheet.Grid, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, grid, ok := s.sheet.EmbeddedGrid(at, s.computeOptions())
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
	var buf bytes.Buffer
	_ = tsvsheet.WriteTSV(&buf, s.sheet.Source())
	return buf.Bytes()
}

// grid deep-copies a grid to a plain [][]string for a State snapshot.
func grid(g tsvsheet.Grid) [][]string {
	out := make([][]string, len(g))
	for i, row := range g {
		out[i] = append([]string(nil), row...)
	}
	return out
}
