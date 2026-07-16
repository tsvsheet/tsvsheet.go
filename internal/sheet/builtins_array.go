package sheet

import (
	"sort"
	"strings"

	"github.com/uplang/tsvsheet.go/internal/tsvt"
)

// evalArray dispatches the dynamic-array builtins, which produce a 2-D result
// that spills. ok is false for any other name.
func (r resolver) evalArray(name funcName, args []tsvt.Expr) (Value, boolResult) {
	switch name {
	case "sequence":
		return r.arraySequence(args), true
	case "transpose":
		return r.arrayTranspose(args), true
	case "unique":
		return r.arrayUnique(args), true
	case "sort":
		return r.arraySort(args), true
	case "filter":
		return r.arrayFilter(args), true
	case "flatten":
		return r.arrayFlatten(args), true
	default:
		return Value{}, false
	}
}

// isArray reports whether name is one of the dynamic-array builtins.
func isArray(name funcName) boolResult {
	switch name {
	case "sequence", "transpose", "unique", "sort", "filter", "flatten":
		return true
	default:
		return false
	}
}

// arraySequence generates rows×cols consecutive numbers from 1: SEQUENCE(rows,
// [cols]).
func (r resolver) arraySequence(args []tsvt.Expr) Value {
	if len(args) < 1 || len(args) > 2 {
		return errorValue(ErrValue)
	}
	rows, cols, bad := r.seqDims(args)
	if bad.isError() {
		return bad
	}
	if rows < 1 || cols < 1 {
		return errorValue(ErrValue)
	}
	if r.comp.limits.tooManyCells(resultDim(rows), resultDim(cols)) {
		return errorValue(ErrValue) // result exceeds the cell budget
	}
	return arrayValue(sequenceMatrix(rows, cols))
}

// seqDims reads SEQUENCE's row count and optional column count (default 1).
func (r resolver) seqDims(args []tsvt.Expr) (charPos, charPos, Value) {
	rows, bad := r.indexArg(args[0])
	if bad.isError() {
		return 0, 0, bad
	}
	if len(args) < 2 {
		return rows, 1, Value{}
	}
	cols, bad := r.indexArg(args[1])
	if bad.isError() {
		return 0, 0, bad
	}
	return rows, cols, Value{}
}

// sequenceMatrix builds a rows×cols grid of consecutive numbers from 1.
func sequenceMatrix(rows, cols charPos) [][]Value {
	m := make([][]Value, rows)
	n := floatVal(1)
	for i := range m {
		m[i] = make([]Value, cols)
		for j := range m[i] {
			m[i][j] = numberValue(n)
			n++
		}
	}
	return m
}

// arrayTranspose swaps the rows and columns of a range.
func (r resolver) arrayTranspose(args []tsvt.Expr) Value {
	if len(args) != 1 {
		return errorValue(ErrValue)
	}
	m := r.argMatrix(args[0])
	out := make([][]Value, len(m[0]))
	for j := range out {
		out[j] = make([]Value, len(m))
		for i := range out[j] {
			out[j][i] = m[i][j]
		}
	}
	return arrayValue(out)
}

// arrayUnique keeps the first occurrence of each distinct row.
func (r resolver) arrayUnique(args []tsvt.Expr) Value {
	if len(args) != 1 {
		return errorValue(ErrValue)
	}
	seen := make(map[string]boolResult)
	var out [][]Value
	for _, row := range r.argMatrix(args[0]) {
		if key := rowKey(row); !seen[key] {
			seen[key] = true
			out = append(out, row)
		}
	}
	return arrayValue(out)
}

// rowKey is a distinctness key for a row of values.
func rowKey(row []Value) string {
	parts := make([]string, len(row))
	for i, v := range row {
		parts[i] = v.String()
	}
	return strings.Join(parts, "\x00")
}

// arraySort sorts a range's rows by their first column, ascending.
func (r resolver) arraySort(args []tsvt.Expr) Value {
	if len(args) != 1 {
		return errorValue(ErrValue)
	}
	out := append([][]Value(nil), r.argMatrix(args[0])...)
	sort.SliceStable(out, func(i, j int) bool { return lessValue(out[i][0], out[j][0]) })
	return arrayValue(out)
}

// lessValue orders two values: numerics by value, otherwise by text.
func lessValue(a, b Value) bool {
	if numericish(a) && numericish(b) {
		return a.num < b.num
	}
	return a.String() < b.String()
}

// arrayFilter keeps the rows of the first range whose parallel condition cell is
// truthy; no match is #N/A.
func (r resolver) arrayFilter(args []tsvt.Expr) Value {
	if len(args) != 2 {
		return errorValue(ErrValue)
	}
	out := filterRows(r.argMatrix(args[0]), flatten1D(r.argMatrix(args[1])))
	if len(out) == 0 {
		return errorValue(ErrNA)
	}
	return arrayValue(out)
}

// filterRows keeps the rows of m whose parallel condition cell is truthy; a
// condition shorter than m leaves the trailing rows unmatched.
func filterRows(m [][]Value, cond []Value) [][]Value {
	var out [][]Value
	for i, row := range m {
		if i >= len(cond) {
			break
		}
		if keep, _ := cond[i].truthy(); keep {
			out = append(out, row)
		}
	}
	return out
}

// arrayFlatten stacks every cell of a range into a single column.
func (r resolver) arrayFlatten(args []tsvt.Expr) Value {
	if len(args) != 1 {
		return errorValue(ErrValue)
	}
	cells := flatten1D(r.argMatrix(args[0]))
	out := make([][]Value, len(cells))
	for i, v := range cells {
		out[i] = []Value{v}
	}
	return arrayValue(out)
}
