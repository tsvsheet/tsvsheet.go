package sheet

import (
	"strings"
	"time"

	"github.com/uplang/tsvsheet.go/internal/tsvt"
)

// Path identifies a sheet to a Loader: the reference written in a
// SHEET(...) call, and (as the loader's result) the sheet's own resolved path.
type Path string

// Loader resolves the sheet referenced by ref, relative to the embedding
// sheet's own path base, returning the parsed sub-sheet and its resolved path
// (used for cycle detection and as the base for the sub-sheet's own SHEET
// calls). The frontend injects it, keeping the engine filesystem-free; a
// resolution or containment failure is reported as an error and surfaces as
// #REF!.
type Loader func(base, ref Path) (Sheet, Path, error)

// embedEnv is the cross-sheet context threaded through a compute pass: the
// loader, the sheet's own path, the arguments passed by the embedding SHEET
// call (read by INPUT), and the set of sheet paths on the embedding stack (for
// cycle detection).
type embedEnv struct {
	loader   Loader
	visiting map[Path]boolResult
	base     Path
	args     []Value
}

// ComputeOptions configures a compute pass. Loader and Base enable embedded
// sub-sheets; a zero Loader disables SHEET (it resolves to #REF!).
type ComputeOptions struct {
	At     time.Time
	Loader Loader
	Base   Path
	Limits Limits
}

// ComputeWith computes the sheet with an injected sheet loader, so SHEET(...)
// cells embed other sheets. The sheet's own Base path seeds cycle detection; the
// injected Limits (or DefaultLimits when unset) bound every allocation.
func (s Sheet) ComputeWith(opts ComputeOptions) Grid {
	env := embedEnv{
		loader:   opts.Loader,
		base:     opts.Base,
		visiting: map[Path]boolResult{opts.Base: true},
	}
	return s.computeGrid(newEmbedComputer(s, opts.At, env, effectiveLimits(opts.Limits)))
}

// newEmbedComputer is newComputer with an embedding environment and the injected
// resource limits attached; embedded and foreign child computers inherit the
// parent's limits so a nested sheet cannot escape the ceiling.
func newEmbedComputer(s Sheet, now time.Time, env embedEnv, limits Limits) computer {
	c := newComputer(s, now)
	c.env = env
	c.limits = limits
	return c
}

// evalEmbed dispatches the cross-sheet builtins SHEET and INPUT (OUTPUT is an
// ordinary identity function). ok is false for any other name.
func (r resolver) evalEmbed(name funcName, args []tsvt.Expr) (Value, boolResult) {
	switch name {
	case "sheet":
		return r.evalSheet(args), true
	case "input":
		return r.evalInput(args), true
	}
	return Value{}, false
}

// isEmbed reports whether name is a lazily-dispatched cross-sheet builtin.
func isEmbed(name funcName) boolResult {
	return boolResult(name == "sheet" || name == "input")
}

// evalInput resolves INPUT(n) to the nth (1-based) argument the embedding
// SHEET(...) call passed; out of range, or in a sheet that was not embedded, is
// #REF!.
func (r resolver) evalInput(args []tsvt.Expr) Value {
	if len(args) != 1 {
		return errorValue(ErrValue)
	}
	num, bad := r.eval(args[0]).asNumber()
	if bad.isError() {
		return bad
	}
	idx := int(num) - 1
	if idx < 0 || idx >= len(r.comp.env.args) {
		return errorValue(ErrRef)
	}
	return r.comp.env.args[idx]
}

// evalSheet loads the sheet named by SHEET(path, args…), computes it, and
// returns its OUTPUT cell's value. A missing loader, a resolution failure, or a
// sheet without a single OUTPUT cell is #REF!; a sheet already on the embedding
// stack is #CIRC!.
func (r resolver) evalSheet(args []tsvt.Expr) Value {
	sub, path, inputs, bad, ok := r.sheetTarget(args)
	if !ok {
		return bad
	}
	return r.embed(sub, path, inputs)
}

// sheetTarget resolves a SHEET(path, args…) call to its sub-sheet, resolved
// path, and evaluated argument values. ok is false on any failure, and bad
// carries the error value to surface (#VALUE! arity, a propagated path error,
// #REF! for a missing loader/unresolved path, or #CIRC! for a cycle).
func (r resolver) sheetTarget(args []tsvt.Expr) (Sheet, Path, []Value, Value, boolResult) {
	if len(args) < 1 {
		return Sheet{}, "", nil, errorValue(ErrValue), false
	}
	pathVal := r.eval(args[0])
	if pathVal.isError() {
		return Sheet{}, "", nil, pathVal, false
	}
	if r.comp.env.loader == nil {
		return Sheet{}, "", nil, errorValue(ErrRef), false
	}
	sub, resolved, err := r.comp.env.loader(r.comp.env.base, Path(pathVal.String()))
	if err != nil {
		return Sheet{}, "", nil, errorValue(ErrRef), false
	}
	if r.comp.env.visiting[resolved] {
		return Sheet{}, "", nil, errorValue(ErrCirc), false
	}
	return sub, resolved, r.argValues(args[1:]), Value{}, true
}

// EmbeddedGrid resolves the sub-sheet embedded by a SHEET(...) cell and returns
// its resolved path and its own computed grid — the projection a frontend
// renders as a nested sheet inside the cell. ok is false when the cell is not a
// top-level SHEET call or the reference cannot be resolved.
func (s Sheet) EmbeddedGrid(at Address, opts ComputeOptions) (Path, Grid, bool) {
	cl, inGrid := s.at(rowIndex(at.Row), colIndex(at.Col))
	if !inGrid || !cl.isFormula() {
		return "", nil, false
	}
	call, isCall := topLevelSheetCall(cl.formula)
	if !isCall {
		return "", nil, false
	}
	visiting := map[Path]boolResult{opts.Base: true}
	limits := effectiveLimits(opts.Limits)
	root := resolver{
		comp: newEmbedComputer(s, opts.At, embedEnv{loader: opts.Loader, base: opts.Base, visiting: visiting}, limits),
	}
	sub, path, inputs, _, ok := root.sheetTarget(call.Args)
	if !ok {
		return "", nil, false
	}
	child := embedEnv{loader: opts.Loader, base: path, args: inputs, visiting: withPath(visiting, path)}
	return path, sub.computeGrid(newEmbedComputer(sub, opts.At, child, limits)), true
}

// topLevelSheetCall reports whether a formula is a bare SHEET(...) call and
// returns it.
func topLevelSheetCall(expr tsvt.Expr) (tsvt.Call, boolResult) {
	call, ok := expr.(tsvt.Call)
	return call, boolResult(ok && strings.EqualFold(call.Name, "sheet"))
}

// embed computes sub as a child sheet (carrying inputs for its INPUT calls and
// the extended visiting set) and returns its OUTPUT cell's value.
func (r resolver) embed(sub Sheet, path Path, inputs []Value) Value {
	out, ok := sub.outputCell()
	if !ok {
		return errorValue(ErrRef)
	}
	child := newEmbedComputer(sub, r.comp.now, embedEnv{
		loader:   r.comp.env.loader,
		base:     path,
		args:     inputs,
		visiting: withPath(r.comp.env.visiting, path),
	}, r.comp.limits)
	return child.read(rowIndex(out.Row), colIndex(out.Col))
}

// outputCell returns the address of the sheet's single OUTPUT cell; ok is false
// when the sheet has no OUTPUT cell or more than one.
func (s Sheet) outputCell() (Address, boolResult) {
	var found Address
	count := 0
	s.eachFormula(func(at Address) {
		if isOutputCall(s.cells[at.Row][at.Col].formula) {
			found, count = at, count+1
		}
	})
	return found, boolResult(count == 1)
}

// isOutputCall reports whether a formula is a top-level OUTPUT(...) call.
func isOutputCall(expr tsvt.Expr) boolResult {
	call, ok := expr.(tsvt.Call)
	return boolResult(ok && strings.EqualFold(call.Name, "output"))
}

// withPath returns set extended with p (copied, so callers keep their own set).
func withPath(set map[Path]boolResult, p Path) map[Path]boolResult {
	next := make(map[Path]boolResult, len(set)+1)
	for k := range set {
		next[k] = true
	}
	next[p] = true
	return next
}

// outputValue is the OUTPUT(expr) builtin: it returns its argument unchanged,
// marking the cell (structurally) as the sheet's embeddable output.
func outputValue(args []Value) Value { return args[0] }
