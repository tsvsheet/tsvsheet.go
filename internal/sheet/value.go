package sheet

import (
	"math"
	"strconv"
)

// ErrorValue is a spreadsheet error value — a cell value, not a Go error. It
// propagates through expressions per ADR 0003 (rules 3, 8, 12, 14).
type ErrorValue string

// The error values. #REF! (out-of-grid), #VALUE! (type), #NAME? (unknown
// function), #DIV/0! (division by zero), #CIRC! (a formula whose evaluation
// depends on itself), #N/A (lookup miss / NA()), #NUM! (numeric domain), #NULL!
// (empty range intersection), #SPILL! (blocked dynamic-array spill).
const (
	ErrRef   ErrorValue = "#REF!"
	ErrValue ErrorValue = "#VALUE!"
	ErrName  ErrorValue = "#NAME?"
	ErrDiv   ErrorValue = "#DIV/0!"
	ErrCirc  ErrorValue = "#CIRC!"
	ErrNA    ErrorValue = "#N/A"
	ErrNum   ErrorValue = "#NUM!"
	ErrNull  ErrorValue = "#NULL!"
	ErrSpill ErrorValue = "#SPILL!"
)

// valueKind tags the inhabited value shapes plus empty.
type valueKind int

const (
	kindEmpty valueKind = iota
	kindNumber
	kindString
	kindBool
	kindDate
	kindError
)

// Value is an evaluated cell value: empty, number, string, or error.
type Value struct {
	str  string
	kind valueKind
	num  float64
}

// emptyValue is the empty cell (§ ADR 0003 rule 8).
func emptyValue() Value { return Value{kind: kindEmpty} }

// numberValue wraps a float result.
func numberValue(n floatVal) Value { return Value{kind: kindNumber, num: float64(n)} }

// stringValue wraps a text result.
func stringValue(s textVal) Value { return Value{kind: kindString, str: string(s)} }

// boolValue wraps a boolean result; the number field carries 1 (true) or 0
// (false) so a bool coerces to a number for free.
func boolValue(isTrue boolResult) Value {
	if isTrue {
		return Value{kind: kindBool, num: 1}
	}
	return Value{kind: kindBool, num: 0}
}

// dateValue wraps a date/time serial number; it renders as an ISO date and
// coerces to its serial in arithmetic.
func dateValue(serial floatVal) Value { return Value{kind: kindDate, num: float64(serial)} }

// errorValue wraps an error value.
func errorValue(e ErrorValue) Value { return Value{kind: kindError, str: string(e)} }

// value parses a raw cell string into a Value: empty stays empty, a numeric
// string becomes a number, a recognized error code round-trips as an error, and
// anything else is a string.
func value(raw textVal) Value {
	if raw == "" {
		return emptyValue()
	}
	if n, err := strconv.ParseFloat(string(raw), 64); err == nil {
		return numberValue(floatVal(n))
	}
	if isErrorCode(raw) {
		return Value{kind: kindError, str: string(raw)}
	}
	return stringValue(raw)
}

// isErrorCode reports whether raw is one of the error values.
func isErrorCode(raw textVal) bool {
	switch ErrorValue(raw) {
	case ErrRef, ErrValue, ErrName, ErrDiv, ErrCirc, ErrNA, ErrNum, ErrNull, ErrSpill:
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
	case kindBool:
		if v.num != 0 {
			return "TRUE"
		}
		return "FALSE"
	case kindDate:
		return renderSerial(floatVal(v.num))
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
	case kindNumber, kindBool, kindDate:
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
	case kindNumber, kindBool, kindDate:
		return v.num != 0, v
	case kindString:
		return v.str != "", v
	default:
		return false, v
	}
}

// round rounds half away from zero to the given number of decimal places.
func round(n floatVal, places decimalPlaces) float64 {
	scale := math.Pow(10, float64(places))
	return math.Round(float64(n)*scale) / scale
}
