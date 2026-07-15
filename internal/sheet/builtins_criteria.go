package sheet

import (
	"strconv"
	"strings"

	"github.com/uplang/tsvsheet.go/internal/tsvt"
)

// evalCriteria dispatches the conditional-aggregate builtins, which pair a range
// with a criterion. ok is false for any other name.
func (r resolver) evalCriteria(name funcName, args []tsvt.Expr) (Value, boolResult) {
	switch name {
	case "countif":
		return r.criteriaCount(args), true
	case "sumif":
		return r.criteriaSum(args, false), true
	case "averageif":
		return r.criteriaSum(args, true), true
	default:
		return Value{}, false
	}
}

// isCriteria reports whether name is one of the conditional-aggregate builtins.
func isCriteria(name funcName) boolResult {
	switch name {
	case "countif", "sumif", "averageif":
		return true
	default:
		return false
	}
}

// criteriaCount implements COUNTIF(range, criterion).
func (r resolver) criteriaCount(args []tsvt.Expr) Value {
	if len(args) != 2 {
		return errorValue(ErrValue)
	}
	cells := flatten1D(r.argMatrix(args[0]))
	crit := r.eval(args[1])
	count := 0
	for _, cell := range cells {
		if matchesCriterion(cell, crit) {
			count++
		}
	}
	return numberValue(floatVal(count))
}

// criteriaSum implements SUMIF/AVERAGEIF(range, criterion, [sumRange]); when a
// sum range is given the matching positions are summed there.
func (r resolver) criteriaSum(args []tsvt.Expr, isAverage boolResult) Value {
	if len(args) < 2 || len(args) > 3 {
		return errorValue(ErrValue)
	}
	cells := flatten1D(r.argMatrix(args[0]))
	sumCells := cells
	if len(args) == 3 {
		sumCells = flatten1D(r.argMatrix(args[2]))
	}
	total, matched, bad := foldMatches(cells, sumCells, r.eval(args[1]))
	if bad.isError() {
		return bad
	}
	if isAverage {
		if matched == 0 {
			return errorValue(ErrDiv)
		}
		return numberValue(floatVal(total / float64(matched)))
	}
	return numberValue(floatVal(total))
}

// foldMatches sums the sumCells at positions whose cells match the criterion,
// reporting the total, the match count, and any error operand in the sum.
func foldMatches(cells, sumCells []Value, criterion Value) (float64, int, Value) {
	total := 0.0
	matched := 0
	for i, cell := range cells {
		if !matchesCriterion(cell, criterion) || i >= len(sumCells) {
			continue
		}
		n, bad := sumCells[i].asNumber()
		if bad.isError() {
			return 0, 0, bad
		}
		total += n
		matched++
	}
	return total, matched, Value{}
}

// matchesCriterion tests a cell against a criterion value: a bare value matches
// by equality; a value prefixed with a comparison operator (>, <, >=, <=, <>, =)
// matches numerically.
func matchesCriterion(cell, criterion Value) boolResult {
	op, operand := parseCriterion(textVal(criterion.String()))
	if op == "" {
		return equalValues(cell, value(operand))
	}
	cellNum, cellBad := cell.asNumber()
	operandNum, err := strconv.ParseFloat(string(operand), 64)
	if cellBad.isError() || err != nil {
		return false
	}
	return boolResult(numberOrder(op, floatVal(cellNum), floatVal(operandNum)))
}

// parseCriterion splits a leading comparison operator from a criterion string;
// the operator is a tsvt.BinaryOp (empty for a bare equality match).
func parseCriterion(crit textVal) (tsvt.BinaryOp, textVal) {
	for _, op := range []tsvt.BinaryOp{tsvt.OpGe, tsvt.OpLe, tsvt.OpNe, tsvt.OpGt, tsvt.OpLt, tsvt.OpEq} {
		if rest, ok := strings.CutPrefix(string(crit), string(op)); ok {
			return op, textVal(rest)
		}
	}
	return "", crit
}
