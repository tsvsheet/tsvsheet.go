package sheet

import "github.com/uplang/tsvsheet.go/internal/tsvt"

// foreignStatus is the outcome of resolving a `"file"!` reference's target sheet.
type foreignStatus int

const (
	foreignOK      foreignStatus = iota
	foreignMissing               // no loader, or the path cannot be loaded → #REF!
	foreignCycle                 // the target is already on the embedding stack → #CIRC!
)

// foreignError maps a non-OK status to the error value the reference yields.
func foreignError(status foreignStatus) ErrorValue {
	if status == foreignCycle {
		return ErrCirc
	}
	return ErrRef
}

// foreign loads and computes the sheet named by a `"file"!` qualifier (resolved
// relative to this computer's own path) and returns a computer to read its cells
// from. A missing loader or unresolved path is foreignMissing; a target already
// on the embedding stack is foreignCycle.
func (r resolver) foreign(file string) (computer, foreignStatus) {
	if r.comp.env.loader == nil {
		return computer{}, foreignMissing
	}
	sub, resolved, err := r.comp.env.loader(r.comp.env.base, Path(file))
	if err != nil {
		return computer{}, foreignMissing
	}
	if r.comp.env.visiting[resolved] {
		return computer{}, foreignCycle
	}
	child := embedEnv{loader: r.comp.env.loader, base: resolved, visiting: withPath(r.comp.env.visiting, resolved)}
	return newEmbedComputer(sub, r.comp.now, child, r.comp.limits), foreignOK
}

// foreignCells resolves a `"file"!` reference to a cellset read from the target
// sheet; an unresolvable target is a single #REF!/#CIRC! value.
func (r resolver) foreignCells(ref tsvt.RangeRef) cellset {
	target, status := r.foreign(ref.File)
	if status != foreignOK {
		return cellset{values: []Value{errorValue(foreignError(status))}, isSingle: boolResult(ref.To == nil)}
	}
	return resolver{comp: target}.resolveOperand(stripFile(ref))
}

// foreignMatrix resolves a `"file"!` range to its rows×columns of values from
// the target sheet; an unresolvable target is a 1×1 #REF!/#CIRC! block.
func (r resolver) foreignMatrix(ref tsvt.RangeRef) [][]Value {
	target, status := r.foreign(ref.File)
	if status != foreignOK {
		return [][]Value{{errorValue(foreignError(status))}}
	}
	return resolver{comp: target}.rangeMatrix(stripFile(ref))
}

// stripFile returns ref addressed at the current sheet — its `"file"!` qualifier
// removed — so a foreign resolver reads it as a local reference.
func stripFile(ref tsvt.RangeRef) tsvt.RangeRef {
	ref.File = ""
	return ref
}
