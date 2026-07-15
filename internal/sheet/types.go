package sheet

// Domain types name the primitive quantities the sheet engine threads through
// its free functions, so a coordinate, a cell value, and a numeric result are
// never bare int/string/float64 at a call boundary.
type (
	// rowIndex is a 0-based grid row.
	rowIndex int
	// colIndex is a 0-based grid column.
	colIndex int
	// gridPos is a grid coordinate on either axis (used where the axis is
	// generic, e.g. ordering a matrix corner).
	gridPos int
	// floatVal is a numeric cell/expression value.
	floatVal float64
	// textVal is raw cell text or a rendered value string.
	textVal string
	// boolResult is a truth value carried across a call boundary.
	boolResult bool
	// columnLetters is a spreadsheet column label such as "AA".
	columnLetters string
	// funcName is a builtin function name (case-insensitive).
	funcName string
	// decimalPlaces is a rounding precision.
	decimalPlaces int
	// argCount is a number of arguments passed to a function call.
	argCount int
)
