package sheet

import "github.com/uplang/tsvsheet.go/internal/tsvt"

// evalIfs evaluates IFS(cond1, val1, …): the value of the first truthy
// condition. A non-paired argument count is #VALUE!; no truthy condition is
// #N/A. An error condition propagates.
func (r resolver) evalIfs(args []tsvt.Expr) Value {
	if len(args) < 2 || len(args)%2 != 0 {
		return errorValue(ErrValue)
	}
	for i := 0; i+1 < len(args); i += 2 {
		chosen, v := r.eval(args[i]).truthy()
		if v.isError() {
			return v
		}
		if chosen {
			return r.eval(args[i+1])
		}
	}
	return errorValue(ErrNA)
}

// evalIferror returns the first argument unless it is an error (IFERROR) or the
// #N/A error (IFNA, isNAOnly), in which case it returns the fallback.
func (r resolver) evalIferror(args []tsvt.Expr, isNAOnly boolResult) Value {
	if len(args) != 2 {
		return errorValue(ErrValue)
	}
	v := r.eval(args[0])
	if caught(v, isNAOnly) {
		return r.eval(args[1])
	}
	return v
}

// caught reports whether v is the kind of error the conditional intercepts.
func caught(v Value, isNAOnly boolResult) boolResult {
	if !v.isError() {
		return false
	}
	if isNAOnly {
		return boolResult(v.str == string(ErrNA))
	}
	return true
}

// evalSwitch evaluates SWITCH(subject, case1, val1, …, [default]): the value
// whose case equals subject, else the trailing default, else #N/A.
func (r resolver) evalSwitch(args []tsvt.Expr) Value {
	if len(args) < 3 {
		return errorValue(ErrValue)
	}
	subject := r.eval(args[0])
	if subject.isError() {
		return subject
	}
	i := 1
	for ; i+1 < len(args); i += 2 {
		if equalValues(subject, r.eval(args[i])) {
			return r.eval(args[i+1])
		}
	}
	if i < len(args) {
		return r.eval(args[i]) // trailing default
	}
	return errorValue(ErrNA)
}

// equalValues reports whether two values are equal under the comparison rules.
func equalValues(a, b Value) boolResult {
	result := compare(tsvt.OpEq, a, b)
	return boolResult(result.kind == kindBool && result.num != 0)
}

// fnTrue and fnFalse are the boolean constants.
func fnTrue([]Value) Value  { return boolValue(true) }
func fnFalse([]Value) Value { return boolValue(false) }

// fnNa is the #N/A error value.
func fnNa([]Value) Value { return errorValue(ErrNA) }

// fnAnd is TRUE iff every operand is truthy; fnOr iff any is. (Error operands
// are short-circuited by the eager dispatcher.)
func fnAnd(args []Value) Value { return logicalFold(args, true) }
func fnOr(args []Value) Value  { return logicalFold(args, false) }

// logicalFold folds the operands' truthiness: isAll=true is AND (FALSE on the
// first falsy), isAll=false is OR (TRUE on the first truthy).
func logicalFold(args []Value, isAll boolResult) Value {
	for _, arg := range args {
		t, _ := arg.truthy()
		if boolResult(t) != isAll {
			return boolValue(!isAll)
		}
	}
	return boolValue(isAll)
}

// fnNot negates its operand's truthiness.
func fnNot(args []Value) Value {
	t, _ := args[0].truthy()
	return boolValue(boolResult(!t))
}

// fnXor is TRUE iff an odd number of operands are truthy.
func fnXor(args []Value) Value {
	count := 0
	for _, arg := range args {
		if t, _ := arg.truthy(); t {
			count++
		}
	}
	return boolValue(boolResult(count%2 == 1))
}

// parityIs tests whether the integer part of a numeric value is odd (or even);
// an error propagates, a non-number is #VALUE!.
func parityIs(v Value, isOdd boolResult) Value {
	if v.isError() {
		return v
	}
	n, nv := v.asNumber()
	if nv.isError() {
		return nv
	}
	parity := int(n) % 2
	if parity < 0 {
		parity = -parity
	}
	return boolValue(boolResult(parity == 1) == isOdd)
}

// inspectN coerces a value to a number (Excel N): a number is itself, a boolean
// 1/0, an error propagates, anything else is 0.
func inspectN(v Value) Value {
	switch v.kind {
	case kindNumber, kindBool:
		return numberValue(floatVal(v.num))
	case kindError:
		return v
	default:
		return numberValue(0)
	}
}

// typeCode is Excel TYPE: 1 number/empty, 2 text, 4 logical, 16 error.
func typeCode(v Value) int {
	switch v.kind {
	case kindString:
		return 2
	case kindBool:
		return 4
	case kindError:
		return 16
	default: // number or empty
		return 1
	}
}
