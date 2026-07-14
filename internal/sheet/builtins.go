package sheet

import (
	"math"
	"strings"
)

// numerics collects the numeric operands of an aggregate: empty cells are
// excluded (ADR 0003 rule 8), an error operand short-circuits (rule 3, ok
// false), and a non-numeric string is #VALUE!.
func numerics(args []Value) (nums []float64, bad Value, ok bool) {
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
	return numberValue(total)
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
	return numberValue(best)
}

// fnCount counts the non-empty operands; an error operand propagates (rule 3).
func fnCount(args []Value) Value {
	count := 0
	for _, arg := range args {
		if arg.isError() {
			return arg
		}
		if arg.kind != kindEmpty {
			count++
		}
	}
	return numberValue(float64(count))
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
	return numberValue(total / float64(len(nums)))
}

// fnAbs is the absolute value of a single numeric operand; a wrong arity is
// #VALUE!.
func fnAbs(args []Value) Value {
	if len(args) != 1 {
		return errorValue(ErrValue)
	}
	n, v := args[0].asNumber()
	if v.isError() {
		return v
	}
	return numberValue(math.Abs(n))
}

// fnRound rounds its first operand to the integer place count of its second
// (default 0); a wrong arity is #VALUE!.
func fnRound(args []Value) Value {
	if len(args) == 0 || len(args) > 2 {
		return errorValue(ErrValue)
	}
	n, v := args[0].asNumber()
	if v.isError() {
		return v
	}
	places, bad, ok := roundPlaces(args)
	if !ok {
		return bad
	}
	return numberValue(round(n, places))
}

// roundPlaces reads the optional second argument as the decimal place count.
func roundPlaces(args []Value) (places int, bad Value, ok bool) {
	if len(args) < 2 {
		return 0, Value{}, true
	}
	p, v := args[1].asNumber()
	if v.isError() {
		return 0, v, false
	}
	return int(p), Value{}, true
}

// fnConcat joins the string forms of its operands; an error operand propagates.
func fnConcat(args []Value) Value {
	var b strings.Builder
	for _, arg := range args {
		if arg.isError() {
			return arg
		}
		b.WriteString(arg.String())
	}
	return stringValue(b.String())
}

// fnLen is the length of a single operand's string form; a wrong arity is
// #VALUE!.
func fnLen(args []Value) Value {
	if len(args) != 1 {
		return errorValue(ErrValue)
	}
	if args[0].isError() {
		return args[0]
	}
	return numberValue(float64(len(args[0].String())))
}
