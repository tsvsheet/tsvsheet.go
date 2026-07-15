package sheet

import (
	"math"
	"strings"

	"github.com/uplang/tsvsheet.go/internal/tsvt"
)

// mod is truncated-toward-zero remainder, defined for negative and fractional
// operands.
func mod(l, r floatVal) floatVal { return floatVal(math.Mod(float64(l), float64(r))) }

// power raises l to the r-th power.
func power(l, r floatVal) floatVal { return floatVal(math.Pow(float64(l), float64(r))) }

// compare applies a comparison, yielding a boolean TRUE/FALSE (ADR 0004 §1):
// numeric when both operands are numeric (a bool compares as its 1/0), and
// lexicographic when both are strings; a mixed pair is #VALUE!.
func compare(op tsvt.BinaryOp, left, right Value) Value {
	if numericish(left) && numericish(right) {
		return boolValue(boolResult(numberOrder(op, floatVal(left.num), floatVal(right.num))))
	}
	if bothText(left, right) {
		return boolValue(boolResult(stringOrder(op, textVal(text(left)), textVal(text(right)))))
	}
	return errorValue(ErrValue)
}

// numericish reports whether a value participates in numeric comparison — a
// number or a boolean (whose 1/0 lives in the number field).
func numericish(v Value) bool { return v.kind == kindNumber || v.kind == kindBool }

// bothText reports whether both operands compare as text (string or empty).
func bothText(left, right Value) bool {
	return textual(left) && textual(right)
}

// textual reports whether a value participates in string comparison.
func textual(v Value) bool { return v.kind == kindString || v.kind == kindEmpty }

// text is a value's comparable string form (empty for the empty value).
func text(v Value) string {
	if v.kind == kindString {
		return v.str
	}
	return ""
}

// numberOrder evaluates a comparison over two numbers.
func numberOrder(op tsvt.BinaryOp, l, r floatVal) bool {
	switch op {
	case tsvt.OpEq:
		return l == r
	case tsvt.OpNe:
		return l != r
	case tsvt.OpLt:
		return l < r
	case tsvt.OpLe:
		return l <= r
	case tsvt.OpGt:
		return l > r
	default: // OpGe
		return l >= r
	}
}

// stringOrder evaluates a comparison over two strings lexicographically.
func stringOrder(op tsvt.BinaryOp, l, r textVal) bool {
	return numberOrder(op, floatVal(strings.Compare(string(l), string(r))), 0)
}

// evalCall dispatches a function call by case-insensitive name (ADR 0004 §2);
// an unknown name is #NAME? and a call outside the function's arity bounds is
// #VALUE!.
func (r resolver) evalCall(call tsvt.Call) Value {
	name := strings.ToLower(call.Name)
	if name == "if" {
		return r.evalIf(call.Args)
	}
	fn, known := functions[name]
	if !known {
		return errorValue(ErrName)
	}
	if !fn.accepts(argCount(len(call.Args))) {
		return errorValue(ErrValue)
	}
	values := r.argValues(call.Args)
	if bad, found := firstError(values); found {
		return bad
	}
	return fn.impl(values)
}

// function is a registered eager builtin: its arity bounds and its impl over
// pre-evaluated, error-free argument values (ADR 0004 §2). Lazy builtins that
// evaluate their own arguments (currently only `if`) are dispatched separately
// so the registry stays a cycle-free var initializer.
type function struct {
	impl    func(args []Value) Value
	minArgs argCount
	maxArgs argCount // negative means variadic (unbounded)
}

// accepts reports whether n arguments fall within the function's arity bounds.
func (f function) accepts(n argCount) bool {
	return n >= f.minArgs && (f.maxArgs < 0 || n <= f.maxArgs)
}

// firstError returns the first error value among values, left to right.
func firstError(values []Value) (Value, boolResult) {
	for _, v := range values {
		if v.isError() {
			return v, true
		}
	}
	return Value{}, false
}

// evalIf evaluates `if(cond, then, else)` lazily: only cond and the selected
// branch are evaluated (ADR 0004 §2). A wrong arity is #VALUE!; an error
// condition propagates.
func (r resolver) evalIf(args []tsvt.Expr) Value {
	if len(args) != 3 {
		return errorValue(ErrValue)
	}
	chosen, v := r.eval(args[0]).truthy()
	if v.isError() {
		return v
	}
	if chosen {
		return r.eval(args[1])
	}
	return r.eval(args[2])
}

// argValues flattens call arguments into their resolved cell values so an
// aggregate sees every cell of a range argument (§11.3).
func (r resolver) argValues(args []tsvt.Expr) []Value {
	values := make([]Value, 0, len(args))
	for _, arg := range args {
		values = append(values, r.argCells(arg)...)
	}
	return values
}

// argCells expands one argument: a bare reference contributes all its resolved
// cells (so `sum(A:H)` sees the whole range); any other expression is one
// scalar value.
func (r resolver) argCells(arg tsvt.Expr) []Value {
	if ref, ok := arg.(tsvt.RefOperand); ok {
		return r.resolveOperand(ref.Ref).values
	}
	return []Value{r.eval(arg)}
}

// functions is the case-insensitive eager builtin registry (ADR 0004 §2); `if`
// is dispatched separately (evalCall/isKnownFunc) because it is lazy, which also
// keeps this a cycle-free var initializer.
var functions = map[string]function{
	"sum":     {impl: fnSum, minArgs: 1, maxArgs: -1},
	"min":     {impl: fnMin, minArgs: 1, maxArgs: -1},
	"max":     {impl: fnMax, minArgs: 1, maxArgs: -1},
	"count":   {impl: fnCount, minArgs: 1, maxArgs: -1},
	"avg":     {impl: fnAvg, minArgs: 1, maxArgs: -1},
	"average": {impl: fnAvg, minArgs: 1, maxArgs: -1},
	"abs":     {impl: fnAbs, minArgs: 1, maxArgs: 1},
	"round":   {impl: fnRound, minArgs: 1, maxArgs: 2},
	"concat":  {impl: fnConcat, minArgs: 1, maxArgs: -1},
	"len":     {impl: fnLen, minArgs: 1, maxArgs: 1},
	"mod":     {impl: fnMod, minArgs: 2, maxArgs: 2},
}
