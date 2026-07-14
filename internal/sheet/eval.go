package sheet

import "github.com/uplang/tsvsheet.go/internal/tsvt"

// eval evaluates a §11 expression to a Value; error values propagate strictly
// (ADR 0003 rule 3), left operand first.
func (r resolver) eval(expr tsvt.Expr) Value {
	switch e := expr.(type) {
	case tsvt.Number:
		return value(e.Text)
	case tsvt.RefOperand:
		return r.resolveOperand(e.Ref).scalar()
	case tsvt.Unary:
		return r.evalUnary(e)
	case tsvt.Binary:
		return r.evalBinary(e)
	default: // tsvt.Call
		return r.evalCall(expr.(tsvt.Call))
	}
}

// evalUnary applies a unary sign; a non-numeric operand is #VALUE!, an error
// propagates.
func (r resolver) evalUnary(e tsvt.Unary) Value {
	n, v := r.eval(e.X).asNumber()
	if v.isError() {
		return v
	}
	if e.Op == tsvt.OpNeg {
		return numberValue(-n)
	}
	return numberValue(n)
}

// evalBinary evaluates a binary operation, propagating an error operand before
// dispatching arithmetic or comparison.
func (r resolver) evalBinary(e tsvt.Binary) Value {
	left := r.eval(e.Left)
	if left.isError() {
		return left
	}
	right := r.eval(e.Right)
	if right.isError() {
		return right
	}
	if isComparison(e.Op) {
		return compare(e.Op, left, right)
	}
	return arithmetic(e.Op, left, right)
}

// isComparison reports whether op is a §11 comparison operator (level 5).
func isComparison(op tsvt.BinaryOp) bool {
	switch op {
	case tsvt.OpEq, tsvt.OpNe, tsvt.OpLt, tsvt.OpLe, tsvt.OpGt, tsvt.OpGe:
		return true
	default:
		return false
	}
}

// arithmetic applies a multiplicative/additive operator over numeric operands
// (ADR 0003 rules 8, 14).
func arithmetic(op tsvt.BinaryOp, left, right Value) Value {
	l, lv := left.asNumber()
	if lv.isError() {
		return lv
	}
	r, rv := right.asNumber()
	if rv.isError() {
		return rv
	}
	return apply(op, l, r)
}

// apply computes a numeric binary result, guarding division/modulo by zero
// (ADR 0003 rule 14).
func apply(op tsvt.BinaryOp, l, r float64) Value {
	switch op {
	case tsvt.OpMul:
		return numberValue(l * r)
	case tsvt.OpAdd:
		return numberValue(l + r)
	case tsvt.OpSub:
		return numberValue(l - r)
	default: // OpDiv, OpMod
		return divide(op, l, r)
	}
}

// divide applies division or modulo, yielding #DIV/0! on a zero divisor.
func divide(op tsvt.BinaryOp, l, r float64) Value {
	if r == 0 {
		return errorValue(ErrDiv)
	}
	if op == tsvt.OpDiv {
		return numberValue(l / r)
	}
	return numberValue(mod(l, r))
}
