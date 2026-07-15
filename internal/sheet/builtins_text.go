package sheet

import (
	"math"
	"regexp"
	"strings"
	"unicode"
)

// intArg reads a value as a (truncated) character position/count; a non-numeric
// value yields the value's #VALUE! error.
func intArg(v Value) (charPos, Value) {
	n, nv := v.asNumber()
	if nv.isError() {
		return 0, nv
	}
	return charPos(math.Trunc(n)), Value{}
}

// argText is the string form of the i-th argument (arguments are error-free by
// the time an eager builtin runs).
func argText(args []Value, i argCount) string { return args[int(i)].String() }

// fnLower and fnUpper change case.
func fnLower(args []Value) Value { return stringValue(textVal(strings.ToLower(argText(args, 0)))) }
func fnUpper(args []Value) Value { return stringValue(textVal(strings.ToUpper(argText(args, 0)))) }

// fnProper capitalizes the first letter of each word, lowercasing the rest.
func fnProper(args []Value) Value {
	afterLetter := false
	proper := strings.Map(func(r rune) rune {
		mapped := unicode.ToUpper(r)
		if afterLetter {
			mapped = unicode.ToLower(r)
		}
		afterLetter = unicode.IsLetter(r)
		return mapped
	}, argText(args, 0))
	return stringValue(textVal(proper))
}

// fnTrim removes leading/trailing spaces and collapses interior runs to one.
func fnTrim(args []Value) Value {
	return stringValue(textVal(strings.Join(strings.Fields(argText(args, 0)), " ")))
}

// fnClean removes non-printable (control) characters.
func fnClean(args []Value) Value {
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, argText(args, 0))
	return stringValue(textVal(cleaned))
}

// fnLeft and fnRight take a prefix/suffix of a given length (default 1).
func fnLeft(args []Value) Value  { return sideChars(args, true) }
func fnRight(args []Value) Value { return sideChars(args, false) }

// sideChars returns the leading (isFromLeft) or trailing n runes of the text.
func sideChars(args []Value, isFromLeft boolResult) Value {
	runes := []rune(argText(args, 0))
	n, bad := countArg(args)
	if bad.isError() {
		return bad
	}
	n = clampPos(n, charPos(len(runes)))
	if isFromLeft {
		return stringValue(textVal(runes[:n]))
	}
	return stringValue(textVal(runes[charPos(len(runes))-n:]))
}

// countArg reads the optional length argument (default 1); a negative count is
// #VALUE!.
func countArg(args []Value) (charPos, Value) {
	if len(args) < 2 {
		return 1, Value{}
	}
	n, bad := intArg(args[1])
	if bad.isError() {
		return 0, bad
	}
	if n < 0 {
		return 0, errorValue(ErrValue)
	}
	return n, Value{}
}

// clampPos bounds a position/count to the available length.
func clampPos(n, length charPos) charPos {
	if n > length {
		return length
	}
	return n
}

// fnMid returns length runes of text starting at the 1-based position start.
func fnMid(args []Value) Value {
	runes := []rune(argText(args, 0))
	start, bad := intArg(args[1])
	if bad.isError() {
		return bad
	}
	length, bad := intArg(args[2])
	if bad.isError() {
		return bad
	}
	if start < 1 || length < 0 {
		return errorValue(ErrValue)
	}
	from := clampPos(start-1, charPos(len(runes)))
	to := clampPos(from+length, charPos(len(runes)))
	return stringValue(textVal(runes[from:to]))
}

// fnRept repeats text a whole number of times; a negative count is #VALUE!.
func fnRept(args []Value) Value {
	n, bad := intArg(args[1])
	if bad.isError() {
		return bad
	}
	if n < 0 {
		return errorValue(ErrValue)
	}
	return stringValue(textVal(strings.Repeat(argText(args, 0), int(n))))
}

// fnExact is TRUE iff two operands have identical (case-sensitive) text.
func fnExact(args []Value) Value {
	return boolValue(boolResult(argText(args, 0) == argText(args, 1)))
}

// fnT is the operand's text if it is text, else the empty string.
func fnT(args []Value) Value {
	if args[0].kind == kindString {
		return args[0]
	}
	return stringValue("")
}

// fnConcatenate joins the text forms of its operands (Excel CONCATENATE).
func fnConcatenate(args []Value) Value {
	var b strings.Builder
	for i := range args {
		_, _ = b.WriteString(argText(args, argCount(i)))
	}
	return stringValue(textVal(b.String()))
}

// fnFind and fnSearch report the 1-based position of a substring; FIND is
// case-sensitive, SEARCH is not. Not found is #VALUE!.
func fnFind(args []Value) Value   { return locate(args, true) }
func fnSearch(args []Value) Value { return locate(args, false) }

// locate finds needle in haystack from an optional 1-based start (default 1).
func locate(args []Value, isCaseSensitive boolResult) Value {
	needle, haystack := argText(args, 0), argText(args, 1)
	if !isCaseSensitive {
		needle, haystack = strings.ToLower(needle), strings.ToLower(haystack)
	}
	start, bad := startArg(args)
	if bad.isError() {
		return bad
	}
	if int(start) > len(haystack) {
		return errorValue(ErrValue)
	}
	idx := strings.Index(haystack[start:], needle)
	if idx < 0 {
		return errorValue(ErrValue)
	}
	return numberValue(floatVal(int(start) + idx + 1))
}

// startArg reads the optional 1-based start position (default 1) as a 0-based
// offset; a start below 1 is #VALUE!.
func startArg(args []Value) (charPos, Value) {
	if len(args) < 3 {
		return 0, Value{}
	}
	n, bad := intArg(args[2])
	if bad.isError() {
		return 0, bad
	}
	if n < 1 {
		return 0, errorValue(ErrValue)
	}
	return n - 1, Value{}
}

// fnSubstitute replaces occurrences of old with new in text; every occurrence
// unless a 1-based instance number is given.
func fnSubstitute(args []Value) Value {
	text, old, replacement := argText(args, 0), argText(args, 1), argText(args, 2)
	if len(args) < 4 {
		return stringValue(textVal(strings.ReplaceAll(text, old, replacement)))
	}
	nth, bad := intArg(args[3])
	if bad.isError() {
		return bad
	}
	return stringValue(substituteNth(textVal(text), textVal(old), textVal(replacement), nth))
}

// substituteNth replaces only the nth (1-based) occurrence of old.
func substituteNth(text, old, replacement textVal, nth charPos) textVal {
	if nth < 1 || old == "" {
		return text
	}
	whole := string(text)
	idx := 0
	for count := charPos(0); ; count++ {
		at := strings.Index(whole[idx:], string(old))
		if at < 0 {
			return text
		}
		idx += at
		if count+1 == nth {
			return textVal(whole[:idx] + string(replacement) + whole[idx+len(old):])
		}
		idx += len(old)
	}
}

// fnReplace replaces length characters of text starting at the 1-based start.
func fnReplace(args []Value) Value {
	runes := []rune(argText(args, 0))
	start, bad := intArg(args[1])
	if bad.isError() {
		return bad
	}
	length, bad := intArg(args[2])
	if bad.isError() {
		return bad
	}
	if start < 1 || length < 0 {
		return errorValue(ErrValue)
	}
	from := clampPos(start-1, charPos(len(runes)))
	to := clampPos(from+length, charPos(len(runes)))
	return stringValue(textVal(string(runes[:from]) + argText(args, 3) + string(runes[to:])))
}

// fnChar and fnCode convert between a character and its code point.
func fnChar(args []Value) Value {
	code, bad := intArg(args[0])
	if bad.isError() {
		return bad
	}
	if code < 1 || int(code) > unicode.MaxRune {
		return errorValue(ErrValue)
	}
	return stringValue(textVal(rune(code)))
}

func fnCode(args []Value) Value {
	runes := []rune(argText(args, 0))
	if len(runes) == 0 {
		return errorValue(ErrValue)
	}
	return numberValue(floatVal(runes[0]))
}

// fnValue parses text as a number; non-numeric text is #VALUE!.
func fnValue(args []Value) Value {
	v := value(textVal(strings.TrimSpace(argText(args, 0))))
	if v.kind == kindNumber {
		return v
	}
	return errorValue(ErrValue)
}

// fnRegexMatch reports whether text matches a regular expression.
func fnRegexMatch(args []Value) Value {
	re, bad := compileRegex(textVal(argText(args, 1)))
	if bad.isError() {
		return bad
	}
	return boolValue(boolResult(re.MatchString(argText(args, 0))))
}

// fnRegexExtract returns the first match of a regular expression; no match is
// #N/A.
func fnRegexExtract(args []Value) Value {
	re, bad := compileRegex(textVal(argText(args, 1)))
	if bad.isError() {
		return bad
	}
	subject := argText(args, 0)
	if !re.MatchString(subject) {
		return errorValue(ErrNA)
	}
	return stringValue(textVal(re.FindString(subject)))
}

// fnRegexReplace replaces every match of a regular expression.
func fnRegexReplace(args []Value) Value {
	re, bad := compileRegex(textVal(argText(args, 1)))
	if bad.isError() {
		return bad
	}
	return stringValue(textVal(re.ReplaceAllString(argText(args, 0), argText(args, 2))))
}

// compileRegex compiles a pattern, reporting an invalid pattern as #VALUE!.
func compileRegex(pattern textVal) (*regexp.Regexp, Value) {
	re, err := regexp.Compile(string(pattern))
	if err != nil {
		return nil, errorValue(ErrValue)
	}
	return re, Value{}
}
