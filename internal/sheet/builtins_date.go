package sheet

import (
	"math"
	"time"
)

// excelEpoch is serial 0 (1899-12-30), so 1900-01-01 is serial 2. tsvsheet uses
// a plain day count from this epoch (no 1900 leap-year bug).
func excelEpoch() time.Time { return time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC) }

// serialToTime converts a date serial to a UTC time.
func serialToTime(serial floatVal) time.Time {
	return excelEpoch().Add(time.Duration(float64(serial) * 24 * float64(time.Hour)))
}

// daySerial is the whole-day serial for a time's calendar date.
func daySerial(t time.Time) floatVal {
	date := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return floatVal(math.Round(date.Sub(excelEpoch()).Hours() / 24))
}

// datetimeSerial is the serial including the time-of-day fraction.
func datetimeSerial(t time.Time) floatVal {
	moment := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
	return floatVal(moment.Sub(excelEpoch()).Hours() / 24)
}

// renderSerial renders a serial as an ISO date, adding a time when the serial
// has a fractional part.
func renderSerial(serial floatVal) string {
	t := serialToTime(serial)
	if float64(serial) == math.Trunc(float64(serial)) {
		return t.Format("2006-01-02")
	}
	return t.Format("2006-01-02 15:04:05")
}

// argTime decodes the first argument as a date serial into a time; a
// non-numeric argument is #VALUE!.
func argTime(args []Value) (time.Time, Value) {
	serial, bad := args[0].asNumber()
	if bad.isError() {
		return time.Time{}, bad
	}
	return serialToTime(floatVal(serial)), Value{}
}

// The date-component builtins decode a serial and read one field.
func fnYear(args []Value) Value   { return dateField(args, timeYear) }
func fnMonth(args []Value) Value  { return dateField(args, timeMonth) }
func fnDay(args []Value) Value    { return dateField(args, timeDay) }
func fnHour(args []Value) Value   { return dateField(args, timeHour) }
func fnMinute(args []Value) Value { return dateField(args, timeMinute) }
func fnSecond(args []Value) Value { return dateField(args, timeSecond) }

func timeYear(t time.Time) floatVal    { return floatVal(t.Year()) }
func timeMonth(t time.Time) floatVal   { return floatVal(t.Month()) }
func timeDay(t time.Time) floatVal     { return floatVal(t.Day()) }
func timeHour(t time.Time) floatVal    { return floatVal(t.Hour()) }
func timeMinute(t time.Time) floatVal  { return floatVal(t.Minute()) }
func timeSecond(t time.Time) floatVal  { return floatVal(t.Second()) }
func timeWeekday(t time.Time) floatVal { return floatVal(int(t.Weekday()) + 1) }

// dateField reads one numeric field from the first argument's date.
func dateField(args []Value, field func(t time.Time) floatVal) Value {
	t, bad := argTime(args)
	if bad.isError() {
		return bad
	}
	return numberValue(field(t))
}

// fnWeekday is the 1-based day of week (Sunday = 1).
func fnWeekday(args []Value) Value { return dateField(args, timeWeekday) }

// fnDate builds a date serial from year, month, day (each normalized).
func fnDate(args []Value) Value {
	parts, bad := threeInts(args)
	if bad.isError() {
		return bad
	}
	t := time.Date(parts[0], time.Month(parts[1]), parts[2], 0, 0, 0, 0, time.UTC)
	return dateValue(daySerial(t))
}

// threeInts reads the first three arguments as integers.
func threeInts(args []Value) ([3]int, Value) {
	var out [3]int
	for i := range out {
		n, bad := intArg(args[i])
		if bad.isError() {
			return out, bad
		}
		out[i] = int(n)
	}
	return out, Value{}
}

// fnEdate is the date the given whole number of months from a serial.
func fnEdate(args []Value) Value {
	t, months, bad := timeAndOffset(args)
	if bad.isError() {
		return bad
	}
	return dateValue(daySerial(t.AddDate(0, months, 0)))
}

// fnEomonth is the last day of the month, offset by whole months.
func fnEomonth(args []Value) Value {
	t, months, bad := timeAndOffset(args)
	if bad.isError() {
		return bad
	}
	first := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, months+1, 0)
	return dateValue(daySerial(first.AddDate(0, 0, -1)))
}

// timeAndOffset reads a serial and a whole-month offset.
func timeAndOffset(args []Value) (time.Time, int, Value) {
	t, bad := argTime(args)
	if bad.isError() {
		return time.Time{}, 0, bad
	}
	months, bad := intArg(args[1])
	if bad.isError() {
		return time.Time{}, 0, bad
	}
	return t, int(months), Value{}
}

// fnDays is the number of days from the second date to the first.
func fnDays(args []Value) Value {
	end, be := args[0].asNumber()
	if be.isError() {
		return be
	}
	start, bs := args[1].asNumber()
	if bs.isError() {
		return bs
	}
	return numberValue(floatVal(daySerial(serialToTime(floatVal(end))) - daySerial(serialToTime(floatVal(start)))))
}

// fnDatevalue parses an ISO date (YYYY-MM-DD) to a serial; a bad date is
// #VALUE!.
func fnDatevalue(args []Value) Value {
	t, err := time.Parse("2006-01-02", argText(args, 0))
	if err != nil {
		return errorValue(ErrValue)
	}
	return dateValue(daySerial(t))
}
