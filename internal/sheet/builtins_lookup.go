package sheet

import "github.com/uplang/tsvsheet.go/internal/tsvt"

// evalTable dispatches the lookup builtins, which need their range argument's
// rows×columns shape. ok is false for any other name.
func (r resolver) evalTable(name funcName, args []tsvt.Expr) (Value, boolResult) {
	switch name {
	case "rows":
		return r.tableDim(args, true), true
	case "columns":
		return r.tableDim(args, false), true
	case "index":
		return r.tableIndex(args), true
	case "match":
		return r.tableMatch(args), true
	case "vlookup":
		return r.tableLookup(args, true), true
	case "hlookup":
		return r.tableLookup(args, false), true
	default:
		return Value{}, false
	}
}

// isTable reports whether name is one of the range-shaped lookup builtins.
func isTable(name funcName) boolResult {
	switch name {
	case "rows", "columns", "index", "match", "vlookup", "hlookup":
		return true
	default:
		return false
	}
}

// indexArg reads an argument as a 1-based integer index.
func (r resolver) indexArg(arg tsvt.Expr) (charPos, Value) {
	n, bad := r.eval(arg).asNumber()
	if bad.isError() {
		return 0, bad
	}
	return charPos(n), Value{}
}

// tableDim returns the row or column count of the first (range) argument.
func (r resolver) tableDim(args []tsvt.Expr, isRows boolResult) Value {
	if len(args) != 1 {
		return errorValue(ErrValue)
	}
	m := r.argMatrix(args[0])
	if isRows {
		return numberValue(floatVal(len(m)))
	}
	return numberValue(floatVal(len(m[0])))
}

// tableIndex returns the cell at (row, col) of a range (col defaults to 1).
func (r resolver) tableIndex(args []tsvt.Expr) Value {
	if len(args) < 2 || len(args) > 3 {
		return errorValue(ErrValue)
	}
	m := r.argMatrix(args[0])
	row, bad := r.indexArg(args[1])
	if bad.isError() {
		return bad
	}
	col := charPos(1)
	if len(args) == 3 {
		col, bad = r.indexArg(args[2])
		if bad.isError() {
			return bad
		}
	}
	if !withinMatrix(m, row, col) {
		return errorValue(ErrRef)
	}
	return m[row-1][col-1]
}

// withinMatrix reports whether (row, col) is a 1-based cell of m.
func withinMatrix(m [][]Value, row, col charPos) boolResult {
	return row >= 1 && int(row) <= len(m) && col >= 1 && int(col) <= len(m[0])
}

// tableMatch returns the 1-based position of key in a range (exact match), else
// #N/A. The optional match-type argument is accepted and ignored.
func (r resolver) tableMatch(args []tsvt.Expr) Value {
	if len(args) < 2 || len(args) > 3 {
		return errorValue(ErrValue)
	}
	key := r.eval(args[0])
	for i, cell := range flatten1D(r.argMatrix(args[1])) {
		if equalValues(key, cell) {
			return numberValue(floatVal(i + 1))
		}
	}
	return errorValue(ErrNA)
}

// flatten1D flattens a matrix (a row or column vector, or a block) row-major.
func flatten1D(m [][]Value) []Value {
	var out []Value
	for _, row := range m {
		out = append(out, row...)
	}
	return out
}

// tableLookup implements VLOOKUP (isVertical) and HLOOKUP: find key in the first
// column/row of the table and return the idx-th cell of that row/column.
func (r resolver) tableLookup(args []tsvt.Expr, isVertical boolResult) Value {
	if len(args) < 3 || len(args) > 4 {
		return errorValue(ErrValue)
	}
	key := r.eval(args[0])
	m := r.argMatrix(args[1])
	idx, bad := r.indexArg(args[2])
	if bad.isError() {
		return bad
	}
	if isVertical {
		return lookupVertical(key, m, idx)
	}
	return lookupHorizontal(key, m, idx)
}

// lookupVertical searches the first column of m for key and returns column idx.
func lookupVertical(key Value, m [][]Value, idx charPos) Value {
	for _, row := range m {
		if equalValues(key, row[0]) {
			return pick(row, idx)
		}
	}
	return errorValue(ErrNA)
}

// lookupHorizontal searches the first row of m for key and returns row idx.
func lookupHorizontal(key Value, m [][]Value, idx charPos) Value {
	for c := range m[0] {
		if equalValues(key, m[0][c]) {
			return pickColumn(m, gridPos(c), idx)
		}
	}
	return errorValue(ErrNA)
}

// pick returns the idx-th (1-based) element of a row, or #REF! if out of range.
func pick(row []Value, idx charPos) Value {
	if idx < 1 || int(idx) > len(row) {
		return errorValue(ErrRef)
	}
	return row[idx-1]
}

// pickColumn returns the idx-th (1-based) cell of column c, or #REF!.
func pickColumn(m [][]Value, c gridPos, idx charPos) Value {
	if idx < 1 || int(idx) > len(m) {
		return errorValue(ErrRef)
	}
	return m[idx-1][c]
}

// fnChoose returns the index-th (1-based) of its trailing arguments (eager).
func fnChoose(args []Value) Value {
	n, bad := args[0].asNumber()
	if bad.isError() {
		return bad
	}
	idx := int(n)
	if idx < 1 || idx >= len(args) {
		return errorValue(ErrValue)
	}
	return args[idx]
}
