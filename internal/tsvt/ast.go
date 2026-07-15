// Package tsvt is the covered seam over the ANTLR-generated formula parser: it
// turns a cell's formula source (the text after its leading `=`) into an
// immutable typed AST — an Expr over A1 references and literals — or a sentinel
// syntax error, and hides every ANTLR type from the rest of the program.
package tsvt

// The two AST interfaces below are sealed: each has an unexported marker method,
// carried by a zero-size embedded struct, so only the node types in this package
// can satisfy it. Consumers walk the AST by type switch; the markers bound each
// switch's variant set at compile time.
type (
	exprMarker      struct{}
	referenceMarker struct{}
)

func (exprMarker) isExpr()           {}
func (referenceMarker) isReference() {}

// Expr is a formula expression (SPECIFICATION §5). The set is sealed.
type Expr interface{ isExpr() }

// BinaryOp is a binary operator.
type BinaryOp string

// The binary operators.
const (
	OpMul BinaryOp = "*"
	OpDiv BinaryOp = "/"
	OpAdd BinaryOp = "+"
	OpSub BinaryOp = "-"
	OpPow BinaryOp = "^"
	OpCat BinaryOp = "&"
	OpEq  BinaryOp = "="
	OpNe  BinaryOp = "<>"
	OpLt  BinaryOp = "<"
	OpLe  BinaryOp = "<="
	OpGt  BinaryOp = ">"
	OpGe  BinaryOp = ">="
)

// UnaryOp is a unary sign operator.
type UnaryOp string

// The unary operators.
const (
	OpNeg UnaryOp = "-"
	OpPos UnaryOp = "+"
)

// Binary is a binary operation.
type Binary struct {
	exprMarker
	Left  Expr
	Right Expr
	Op    BinaryOp
}

// Unary is a unary sign operation.
type Unary struct {
	exprMarker
	X  Expr
	Op UnaryOp
}

// Percent is a postfix-percent operation: `50%` is `Percent{50}` = 0.5.
type Percent struct {
	exprMarker
	X Expr
}

// Call is a function call; Name is case-preserved (identity resolves
// case-insensitively in the evaluator).
type Call struct {
	exprMarker
	Name string
	Args []Expr
}

// RefOperand is an A1 reference used as an expression operand.
type RefOperand struct {
	exprMarker
	Ref Reference
}

// Number is a numeric literal; Text preserves the source form.
type Number struct {
	exprMarker
	Text string
}

// StringLit is a double-quoted string literal.
type StringLit struct {
	exprMarker
	Value string
}

// BoolLit is a TRUE/FALSE literal.
type BoolLit struct {
	exprMarker
	IsTrue bool
}

// ErrorLit is an error-value literal (`#N/A`, `#REF!`, …); Code is its text.
type ErrorLit struct {
	exprMarker
	Code string
}

// Reference is an A1 reference (SPECIFICATION §4). The set is sealed.
type Reference interface{ isReference() }

// RangeRef is a single A1 cell (To nil) or a rectangular range of two cells.
type RangeRef struct {
	referenceMarker
	To   *CellRef
	From CellRef
}

// CellRef is an A1 cell: a column label and a 1-based row. The `$` absolute
// markers are accepted by the grammar but carry no positional difference in a
// flat grid, so they are not retained.
type CellRef struct {
	Col string
	Row int
}
