package sheet

import (
	"math"
	"strconv"
)

// ErrorValue is a spreadsheet error value — a cell value, not a Go error. It
// propagates through expressions per ADR 0003 (rules 3, 8, 12, 14).
type ErrorValue string

// The error values. #REF! (out-of-grid), #VALUE! (type), #NAME? (unknown
// function / unbound name), #DIV/0! (division or modulo by zero).
const (
	ErrRef   ErrorValue = "#REF!"
	ErrValue ErrorValue = "#VALUE!"
	ErrName  ErrorValue = "#NAME?"
	ErrDiv   ErrorValue = "#DIV/0!"
)

// valueKind tags the three inhabited value shapes plus empty.
type valueKind int

const (
	kindEmpty valueKind = iota
	kindNumber
	kindString
	kindError
)

// Value is an evaluated cell value: empty, number, string, or error.
type Value struct {
	kind valueKind
	num  float64
	str  string
}

// emptyValue is the empty cell (§ ADR 0003 rule 8).
func emptyValue() Value { return Value{kind: kindEmpty} }

// numberValue wraps a float result.
func numberValue(n float64) Value { return Value{kind: kindNumber, num: n} }

// stringValue wraps a text result.
func stringValue(s string) Value { return Value{kind: kindString, str: s} }

// errorValue wraps an error value.
func errorValue(e ErrorValue) Value { return Value{kind: kindError, str: string(e)} }

// value parses a raw cell string into a Value: empty stays empty, a numeric
// string becomes a number, a recognized error code round-trips as an error, and
// anything else is a string.
func value(raw string) Value {
	if raw == "" {
		return emptyValue()
	}
	if n, err := strconv.ParseFloat(raw, 64); err == nil {
		return numberValue(n)
	}
	if isErrorCode(raw) {
		return Value{kind: kindError, str: raw}
	}
	return stringValue(raw)
}

// isErrorCode reports whether raw is one of the four error values.
func isErrorCode(raw string) bool {
	switch ErrorValue(raw) {
	case ErrRef, ErrValue, ErrName, ErrDiv:
		return true
	default:
		return false
	}
}

// isError reports whether the value is an error value.
func (v Value) isError() bool { return v.kind == kindError }

// String renders a Value as its cell text: empty is "", a number is formatted
// without a trailing zero fraction, a string is itself, an error is its code.
func (v Value) String() string {
	switch v.kind {
	case kindNumber:
		return strconv.FormatFloat(v.num, 'f', -1, 64)
	case kindString, kindError:
		return v.str
	default:
		return ""
	}
}

// asNumber coerces the value to a float for arithmetic: empty is 0, a number is
// itself, and a string is #VALUE! (a string here is non-numeric by
// construction — value() parses every numeric string as a number). An error
// value propagates unchanged.
func (v Value) asNumber() (float64, Value) {
	switch v.kind {
	case kindEmpty:
		return 0, emptyValue()
	case kindNumber:
		return v.num, v
	case kindError:
		return 0, v
	default: // kindString
		return 0, errorValue(ErrValue)
	}
}

// truthy evaluates §-`if` truthiness (ADR 0003 rule 9): a number is true iff
// non-zero, a string iff non-empty, empty is false; an error propagates via the
// returned Value (ok=false).
func (v Value) truthy() (bool, Value) {
	switch v.kind {
	case kindError:
		return false, v
	case kindNumber:
		return v.num != 0, v
	case kindString:
		return v.str != "", v
	default:
		return false, v
	}
}

// round rounds half away from zero to the given number of decimal places.
func round(n float64, places int) float64 {
	scale := math.Pow(10, float64(places))
	return math.Round(n*scale) / scale
}
