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
func numericish(v Value) bool {
	return v.kind == kindNumber || v.kind == kindBool || v.kind == kindDate
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
	name := funcName(strings.ToLower(call.Name))
	if v, ok := r.evalLazy(name, call.Args); ok {
		return v
	}
	fn, known := functions[string(name)]
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

// evalLazy dispatches the builtins that evaluate their own arguments — the
// selective conditionals and the single-argument inspectors, which must observe
// errors and empties rather than have them short-circuited by the eager path.
// ok is false for any other (eager) name.
func (r resolver) evalLazy(name funcName, args []tsvt.Expr) (Value, boolResult) {
	if v, ok := r.evalConditional(name, args); ok {
		return v, true
	}
	if v, ok := r.evalClock(name, args); ok {
		return v, true
	}
	return r.evalInspector(name, args)
}

// evalClock dispatches the volatile clock builtins TODAY and NOW, which read the
// pass clock; ok is false for any other name. A non-empty argument list is
// #VALUE!.
func (r resolver) evalClock(name funcName, args []tsvt.Expr) (Value, boolResult) {
	switch name {
	case "today":
		return clockResult(argCount(len(args)), dateValue(daySerial(r.comp.now))), true
	case "now":
		return clockResult(argCount(len(args)), dateValue(datetimeSerial(r.comp.now))), true
	default:
		return Value{}, false
	}
}

// clockResult returns v for a no-argument call, else #VALUE!.
func clockResult(argc argCount, v Value) Value {
	if argc != 0 {
		return errorValue(ErrValue)
	}
	return v
}

// evalConditional handles the selectively-lazy conditionals, which evaluate
// only the arguments they need. ok is false for a non-conditional name.
func (r resolver) evalConditional(name funcName, args []tsvt.Expr) (Value, boolResult) {
	switch name {
	case "if":
		return r.evalIf(args), true
	case "ifs":
		return r.evalIfs(args), true
	case "iferror":
		return r.evalIferror(args, false), true
	case "ifna":
		return r.evalIferror(args, true), true
	case "switch":
		return r.evalSwitch(args), true
	default:
		return Value{}, false
	}
}

// isConditional reports whether name is one of the lazy conditional builtins.
func isConditional(name funcName) boolResult {
	switch name {
	case "if", "ifs", "iferror", "ifna", "switch":
		return true
	default:
		return false
	}
}

// evalInspector handles the single-argument inspectors (`IS*`, `N`, `TYPE`): it
// evaluates the argument (observing an error or empty result) and applies the
// pure inspector function.
func (r resolver) evalInspector(name funcName, args []tsvt.Expr) (Value, boolResult) {
	fn, ok := inspectors[string(name)]
	if !ok {
		return Value{}, false
	}
	if len(args) != 1 {
		return errorValue(ErrValue), true
	}
	return fn(r.eval(args[0])), true
}

// inspectors are the pure single-argument value functions behind the `IS*`,
// `N`, and `TYPE` builtins. They take an already-evaluated value, so this map
// holds no reference back into evalCall and stays a cycle-free var initializer.
var inspectors = map[string]func(v Value) Value{
	"isblank":   func(v Value) Value { return boolValue(v.kind == kindEmpty) },
	"iserror":   func(v Value) Value { return boolValue(boolResult(v.isError())) },
	"iserr":     func(v Value) Value { return boolValue(boolResult(v.isError()) && v.str != string(ErrNA)) },
	"isna":      func(v Value) Value { return boolValue(boolResult(v.isError()) && v.str == string(ErrNA)) },
	"isnumber":  func(v Value) Value { return boolValue(v.kind == kindNumber) },
	"istext":    func(v Value) Value { return boolValue(v.kind == kindString) },
	"isnontext": func(v Value) Value { return boolValue(v.kind != kindString) },
	"islogical": func(v Value) Value { return boolValue(v.kind == kindBool) },
	"iseven":    func(v Value) Value { return parityIs(v, false) },
	"isodd":     func(v Value) Value { return parityIs(v, true) },
	"n":         inspectN,
	"type":      func(v Value) Value { return numberValue(floatVal(typeCode(v))) },
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

	// Phase 1 — math & trig.
	"pi":       {impl: fnPi, minArgs: 0, maxArgs: 0},
	"sign":     {impl: unaryNumeric(sign), minArgs: 1, maxArgs: 1},
	"int":      {impl: unaryNumeric(mFloor), minArgs: 1, maxArgs: 1},
	"trunc":    {impl: unaryNumeric(mTrunc), minArgs: 1, maxArgs: 1},
	"sqrt":     {impl: unaryNumeric(mSqrt), minArgs: 1, maxArgs: 1},
	"sqrtpi":   {impl: unaryNumeric(sqrtPi), minArgs: 1, maxArgs: 1},
	"power":    {impl: binaryNumeric(mPow), minArgs: 2, maxArgs: 2},
	"exp":      {impl: unaryNumeric(mExp), minArgs: 1, maxArgs: 1},
	"ln":       {impl: unaryNumeric(mLn), minArgs: 1, maxArgs: 1},
	"log10":    {impl: unaryNumeric(mLog10), minArgs: 1, maxArgs: 1},
	"log":      {impl: fnLog, minArgs: 1, maxArgs: 2},
	"quotient": {impl: fnQuotient, minArgs: 2, maxArgs: 2},
	"product":  {impl: fnProduct, minArgs: 1, maxArgs: -1},
	"sumsq":    {impl: fnSumsq, minArgs: 1, maxArgs: -1},
	"sin":      {impl: unaryNumeric(mSin), minArgs: 1, maxArgs: 1},
	"cos":      {impl: unaryNumeric(mCos), minArgs: 1, maxArgs: 1},
	"tan":      {impl: unaryNumeric(mTan), minArgs: 1, maxArgs: 1},
	"asin":     {impl: unaryNumeric(mAsin), minArgs: 1, maxArgs: 1},
	"acos":     {impl: unaryNumeric(mAcos), minArgs: 1, maxArgs: 1},
	"atan":     {impl: unaryNumeric(mAtan), minArgs: 1, maxArgs: 1},
	"atan2":    {impl: binaryNumeric(atan2Excel), minArgs: 2, maxArgs: 2},
	"sinh":     {impl: unaryNumeric(mSinh), minArgs: 1, maxArgs: 1},
	"cosh":     {impl: unaryNumeric(mCosh), minArgs: 1, maxArgs: 1},
	"tanh":     {impl: unaryNumeric(mTanh), minArgs: 1, maxArgs: 1},
	"degrees":  {impl: unaryNumeric(toDegrees), minArgs: 1, maxArgs: 1},
	"radians":  {impl: unaryNumeric(toRadians), minArgs: 1, maxArgs: 1},

	// Phase 2 — logical (eager; conditionals and inspectors dispatch lazily).
	"and":   {impl: fnAnd, minArgs: 1, maxArgs: -1},
	"or":    {impl: fnOr, minArgs: 1, maxArgs: -1},
	"xor":   {impl: fnXor, minArgs: 1, maxArgs: -1},
	"not":   {impl: fnNot, minArgs: 1, maxArgs: 1},
	"true":  {impl: fnTrue, minArgs: 0, maxArgs: 0},
	"false": {impl: fnFalse, minArgs: 0, maxArgs: 0},
	"na":    {impl: fnNa, minArgs: 0, maxArgs: 0},

	// Phase 3 — text.
	"lower":        {impl: fnLower, minArgs: 1, maxArgs: 1},
	"upper":        {impl: fnUpper, minArgs: 1, maxArgs: 1},
	"proper":       {impl: fnProper, minArgs: 1, maxArgs: 1},
	"trim":         {impl: fnTrim, minArgs: 1, maxArgs: 1},
	"clean":        {impl: fnClean, minArgs: 1, maxArgs: 1},
	"left":         {impl: fnLeft, minArgs: 1, maxArgs: 2},
	"right":        {impl: fnRight, minArgs: 1, maxArgs: 2},
	"mid":          {impl: fnMid, minArgs: 3, maxArgs: 3},
	"rept":         {impl: fnRept, minArgs: 2, maxArgs: 2},
	"exact":        {impl: fnExact, minArgs: 2, maxArgs: 2},
	"t":            {impl: fnT, minArgs: 1, maxArgs: 1},
	"concatenate":  {impl: fnConcatenate, minArgs: 1, maxArgs: -1},
	"find":         {impl: fnFind, minArgs: 2, maxArgs: 3},
	"search":       {impl: fnSearch, minArgs: 2, maxArgs: 3},
	"substitute":   {impl: fnSubstitute, minArgs: 3, maxArgs: 4},
	"replace":      {impl: fnReplace, minArgs: 4, maxArgs: 4},
	"char":         {impl: fnChar, minArgs: 1, maxArgs: 1},
	"unichar":      {impl: fnChar, minArgs: 1, maxArgs: 1},
	"code":         {impl: fnCode, minArgs: 1, maxArgs: 1},
	"unicode":      {impl: fnCode, minArgs: 1, maxArgs: 1},
	"value":        {impl: fnValue, minArgs: 1, maxArgs: 1},
	"regexmatch":   {impl: fnRegexMatch, minArgs: 2, maxArgs: 2},
	"regexextract": {impl: fnRegexExtract, minArgs: 2, maxArgs: 2},
	"regexreplace": {impl: fnRegexReplace, minArgs: 3, maxArgs: 3},

	// Phase 4 — date & time (TODAY/NOW dispatch via the clock path).
	"year":      {impl: fnYear, minArgs: 1, maxArgs: 1},
	"month":     {impl: fnMonth, minArgs: 1, maxArgs: 1},
	"day":       {impl: fnDay, minArgs: 1, maxArgs: 1},
	"hour":      {impl: fnHour, minArgs: 1, maxArgs: 1},
	"minute":    {impl: fnMinute, minArgs: 1, maxArgs: 1},
	"second":    {impl: fnSecond, minArgs: 1, maxArgs: 1},
	"weekday":   {impl: fnWeekday, minArgs: 1, maxArgs: 2},
	"date":      {impl: fnDate, minArgs: 3, maxArgs: 3},
	"edate":     {impl: fnEdate, minArgs: 2, maxArgs: 2},
	"eomonth":   {impl: fnEomonth, minArgs: 2, maxArgs: 2},
	"days":      {impl: fnDays, minArgs: 2, maxArgs: 2},
	"datevalue": {impl: fnDatevalue, minArgs: 1, maxArgs: 1},
}
