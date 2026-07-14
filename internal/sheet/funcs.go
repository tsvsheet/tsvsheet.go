package sheet

import (
	"math"
	"strings"

	"github.com/uplang/tsvsheet.go/internal/tsvt"
)

// mod is truncated-toward-zero remainder, defined for negative and fractional
// operands (ADR 0003 rule 14).
func mod(l, r float64) float64 { return math.Mod(l, r) }

// compare applies a §11 comparison, yielding 1 (true) or 0 (false): numeric
// when both operands are numeric, lexicographic when both are strings, #VALUE!
// for mixed (ADR 0003 rule 12).
func compare(op tsvt.BinaryOp, left, right Value) Value {
	if left.kind == kindNumber && right.kind == kindNumber {
		return boolValue(numberOrder(op, left.num, right.num))
	}
	if bothText(left, right) {
		return boolValue(stringOrder(op, text(left), text(right)))
	}
	return errorValue(ErrValue)
}

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

// boolValue maps a Go bool to the 1/0 numeric result of a comparison.
func boolValue(b bool) Value {
	if b {
		return numberValue(1)
	}
	return numberValue(0)
}

// numberOrder evaluates a comparison over two numbers.
func numberOrder(op tsvt.BinaryOp, l, r float64) bool {
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
func stringOrder(op tsvt.BinaryOp, l, r string) bool {
	return numberOrder(op, float64(strings.Compare(l, r)), 0)
}

// evalCall dispatches a function call by case-insensitive name (§11.3); an
// unknown name is #NAME? (ADR 0003 rule 10). `if` is handled separately because
// it evaluates its branches lazily (rule 3), so it must not pre-evaluate args.
func (r resolver) evalCall(call tsvt.Call) Value {
	name := strings.ToLower(call.Name)
	if name == "if" {
		return r.evalIf(call.Args)
	}
	fn, known := functions[name]
	if !known {
		return errorValue(ErrName)
	}
	return fn(r.argValues(call.Args))
}

// evalIf evaluates `if(cond, a, b)` lazily: only cond and the selected branch
// are evaluated (ADR 0003 rule 3). A wrong arity is #VALUE!.
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

// builtin is a function over already-evaluated argument values.
type builtin func(args []Value) Value

// aggregateArgs flattens call arguments into their resolved cell values so an
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

// functions is the case-insensitive builtin set (ADR 0003 rule 10). `if` is
// dispatched separately in evalCall because it is lazy.
var functions = map[string]builtin{
	"sum":    fnSum,
	"min":    fnMin,
	"max":    fnMax,
	"count":  fnCount,
	"avg":    fnAvg,
	"abs":    fnAbs,
	"round":  fnRound,
	"concat": fnConcat,
	"len":    fnLen,
}
