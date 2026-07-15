package sheet

import (
	"math"
	"strings"
)

// numerics collects the numeric operands of an aggregate: empty cells are
// excluded (ADR 0003 rule 8), an error operand short-circuits (rule 3, ok
// false), and a non-numeric string is #VALUE!.
func numerics(args []Value) (nums []float64, bad Value, isOK bool) {
	for _, arg := range args {
		if arg.kind == kindEmpty {
			continue
		}
		n, v := arg.asNumber()
		if v.isError() {
			return nil, v, false
		}
		nums = append(nums, n)
	}
	return nums, Value{}, true
}

// fnSum totals its numeric operands; an empty set is 0.
func fnSum(args []Value) Value {
	nums, bad, ok := numerics(args)
	if !ok {
		return bad
	}
	total := 0.0
	for _, n := range nums {
		total += n
	}
	return numberValue(floatVal(total))
}

// fnMin is the least numeric operand; an empty set is #VALUE!.
func fnMin(args []Value) Value {
	return extreme(args, math.Min)
}

// fnMax is the greatest numeric operand; an empty set is #VALUE!.
func fnMax(args []Value) Value {
	return extreme(args, math.Max)
}

// extreme folds the numeric operands with pick (min or max); an empty numeric
// set has no extreme and is #VALUE!.
func extreme(args []Value, pick func(a, b float64) float64) Value {
	nums, bad, ok := numerics(args)
	if !ok {
		return bad
	}
	if len(nums) == 0 {
		return errorValue(ErrValue)
	}
	best := nums[0]
	for _, n := range nums[1:] {
		best = pick(best, n)
	}
	return numberValue(floatVal(best))
}

// fnCount counts the non-empty operands (error operands are short-circuited by
// the eager dispatcher before they reach here). COUNTA-style; COUNT's
// numbers-only variant arrives with the statistical phase.
func fnCount(args []Value) Value {
	count := 0
	for _, arg := range args {
		if arg.kind != kindEmpty {
			count++
		}
	}
	return numberValue(floatVal(count))
}

// fnAvg is the mean of the numeric operands; an empty set is #DIV/0!.
func fnAvg(args []Value) Value {
	nums, bad, ok := numerics(args)
	if !ok {
		return bad
	}
	if len(nums) == 0 {
		return errorValue(ErrDiv)
	}
	total := 0.0
	for _, n := range nums {
		total += n
	}
	return numberValue(floatVal(total / float64(len(nums))))
}

// fnAbs is the absolute value of a single numeric operand; a non-numeric operand
// is #VALUE!. Arity is enforced by the registry.
func fnAbs(args []Value) Value {
	n, v := args[0].asNumber()
	if v.isError() {
		return v
	}
	return numberValue(floatVal(math.Abs(n)))
}

// fnRound rounds its first operand to the integer place count of its second
// (default 0). Arity (1 or 2) is enforced by the registry.
func fnRound(args []Value) Value {
	n, v := args[0].asNumber()
	if v.isError() {
		return v
	}
	places, bad, ok := roundPlaces(args)
	if !ok {
		return bad
	}
	return numberValue(floatVal(round(floatVal(n), places)))
}

// roundPlaces reads the optional second argument as the decimal place count.
func roundPlaces(args []Value) (places decimalPlaces, bad Value, isOK bool) {
	if len(args) < 2 {
		return 0, Value{}, true
	}
	p, v := args[1].asNumber()
	if v.isError() {
		return 0, v, false
	}
	return decimalPlaces(p), Value{}, true
}

// fnConcat joins the string forms of its operands (error operands are
// short-circuited by the eager dispatcher).
func fnConcat(args []Value) Value {
	var b strings.Builder
	for _, arg := range args {
		_, _ = b.WriteString(arg.String())
	}
	return stringValue(textVal(b.String()))
}

// fnLen is the length of a single operand's string form. Arity is enforced by
// the registry; errors are short-circuited before they reach here.
func fnLen(args []Value) Value {
	return numberValue(floatVal(len(args[0].String())))
}

// fnMod is the remainder of dividing the first operand by the second (Excel MOD,
// which replaces the retired binary `%`); a zero divisor is #DIV/0! and a
// non-numeric operand is #VALUE!.
func fnMod(args []Value) Value {
	dividend, dv := args[0].asNumber()
	if dv.isError() {
		return dv
	}
	divisor, vv := args[1].asNumber()
	if vv.isError() {
		return vv
	}
	if divisor == 0 {
		return errorValue(ErrDiv)
	}
	return numberValue(mod(floatVal(dividend), floatVal(divisor)))
}
