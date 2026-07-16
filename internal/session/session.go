// Package session is the stateful editing model shared by every interactive
// frontend (serve, tui): one spreadsheet — a .tsvt grid of literal and formula
// cells — recomputed through the engine (internal/sheet) after each edit. It is
// the repo's one sanctioned pointer-receiver type: it wraps mutable state
// guarded for concurrent use, so a single Session backs the HTTP handlers and
// the TUI model alike.
package session

import (
	"bytes"
	"sync"
	"time"

	"github.com/uplang/tsvsheet.go/internal/sheet"
)

// State is the complete read model a frontend renders: the computed value grid,
// the cell source texts (literals and "=formulas") for editing, static
// diagnostics, and the dirty flag. It is a value snapshot; mutating it never
// affects the Session.
type State struct {
	Computed    [][]string         `json:"computed"`
	Source      [][]string         `json:"source"`
	Diagnostics []sheet.Diagnostic `json:"diagnostics"`
	IsDirty     bool               `json:"dirty"`
}

// Session is a mutable spreadsheet. Its methods are safe for concurrent use.
type Session struct {
	loader      sheet.Loader
	base        sheet.Path
	sheet       sheet.Sheet
	computed    sheet.Grid
	diagnostics []sheet.Diagnostic
	limits      sheet.Limits
	mu          sync.Mutex
	isDirty     bool
}

// New parses spreadsheet source and computes it eagerly, with no sheet loader —
// SHEET(...) cells resolve to #REF! — and the engine's generous DefaultLimits.
// It fails on a syntax error; the resulting session is clean (not dirty).
func New(src []byte) (*Session, error) {
	return NewEmbeddable(src, nil, "", sheet.DefaultLimits())
}

// NewEmbeddable is New with an injected sheet loader, this sheet's own path (so
// SHEET(...) cells embed other sheets resolved through loader), and the resource
// limits the session enforces on every compute and edit.
func NewEmbeddable(src []byte, loader sheet.Loader, base sheet.Path, limits sheet.Limits) (*Session, error) {
	parsed, err := sheet.Parse(src)
	if err != nil {
		return nil, err
	}
	s := &Session{sheet: parsed, loader: loader, base: base, limits: withDefaults(limits)}
	s.recompute()
	return s, nil
}

// withDefaults resolves the injected limits, falling the zero value (an
// unspecified Limits) back to the engine's generous DefaultLimits so a session
// never enforces a degenerate zero grid dimension.
func withDefaults(limits sheet.Limits) sheet.Limits {
	if limits == (sheet.Limits{}) {
		return sheet.DefaultLimits()
	}
	return limits
}

// recompute re-evaluates the current sheet and refreshes the read model. It uses
// the injected loader (nil disables embedding), the session limits, and samples
// the clock once.
func (s *Session) recompute() {
	s.computed = s.sheet.ComputeWith(s.computeOptions())
	s.diagnostics = sheet.Check(s.sheet)
}

// computeOptions builds the compute options for this session: its loader, base
// path, and resource limits, with the clock sampled at call time.
func (s *Session) computeOptions() sheet.ComputeOptions {
	return sheet.ComputeOptions{At: time.Now(), Loader: s.loader, Base: s.base, Limits: s.limits}
}

// SetCell edits one cell's source text (a literal or a formula) and recomputes.
// A malformed formula is rejected and the sheet is left unchanged (atomic); on
// success the session is marked dirty.
func (s *Session) SetCell(at sheet.Address, text string) error {
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
func (s *Session) structuralEdit(edit func(sheet.Sheet) sheet.Sheet) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sheet = edit(s.sheet)
	s.isDirty = true
	s.recompute()
}

// InsertRow inserts a blank row before at.Row, shifting references down.
func (s *Session) InsertRow(at sheet.Address) {
	s.structuralEdit(func(sh sheet.Sheet) sheet.Sheet { return sh.InsertRow(at) })
}

// DeleteRow removes row at.Row, turning references to it into #REF!.
func (s *Session) DeleteRow(at sheet.Address) {
	s.structuralEdit(func(sh sheet.Sheet) sheet.Sheet { return sh.DeleteRow(at) })
}

// InsertCol inserts a blank column before at.Col, shifting references right.
func (s *Session) InsertCol(at sheet.Address) {
	s.structuralEdit(func(sh sheet.Sheet) sheet.Sheet { return sh.InsertCol(at) })
}

// DeleteCol removes column at.Col, turning references to it into #REF!.
func (s *Session) DeleteCol(at sheet.Address) {
	s.structuralEdit(func(sh sheet.Sheet) sheet.Sheet { return sh.DeleteCol(at) })
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
		Diagnostics: append([]sheet.Diagnostic(nil), s.diagnostics...),
		IsDirty:     s.isDirty,
	}
}

// IsVolatile reports whether the sheet contains clock-dependent functions
// (TODAY/NOW/ISNOW), so a frontend can enable periodic recomputation.
func (s *Session) IsVolatile() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sheet.IsVolatile()
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

// Explain traces how the cell at addr was produced over the current sheet.
func (s *Session) Explain(addr sheet.Address) (sheet.Trace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return sheet.Explain(s.sheet, addr)
}

// References returns the cell at addr's precedents (the spans its formula reads)
// and dependents (the cells whose formulas read it) — the dependency edges a
// frontend highlights on selection.
func (s *Session) References(addr sheet.Address) ([]sheet.Span, []sheet.Address) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sheet.Precedents(addr), s.sheet.Dependents(addr)
}

// Embedded returns the sub-sheet a SHEET(...) cell embeds: its resolved path and
// its own computed grid, for nested rendering. ok is false when the cell is not
// a SHEET call or the reference cannot be resolved.
func (s *Session) Embedded(at sheet.Address) (sheet.Path, sheet.Grid, bool) {
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
	_ = sheet.WriteTSV(&buf, s.sheet.Source())
	return buf.Bytes()
}

// grid deep-copies a grid to a plain [][]string for a State snapshot.
func grid(g sheet.Grid) [][]string {
	out := make([][]string, len(g))
	for i, row := range g {
		out[i] = append([]string(nil), row...)
	}
	return out
}
