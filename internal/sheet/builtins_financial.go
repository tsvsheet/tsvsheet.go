package sheet

import "math"

// argNum reads the i-th argument as a number, or 0 when it is absent.
func argNum(args []Value, i argCount) (floatVal, Value) {
	if int(i) >= len(args) {
		return 0, Value{}
	}
	n, bad := args[int(i)].asNumber()
	if bad.isError() {
		return 0, bad
	}
	return floatVal(n), Value{}
}

// growth is (1+rate)^nper, the compounding factor.
func growth(rate, nper floatVal) floatVal {
	return floatVal(math.Pow(float64(1+rate), float64(nper)))
}

// fnPmt is the periodic payment of a loan: PMT(rate, nper, pv, [fv], [type]).
func fnPmt(args []Value) Value {
	rate, e := argNum(args, 0)
	nper, e2 := argNum(args, 1)
	pv, e3 := argNum(args, 2)
	fv, e4 := argNum(args, 3)
	typ, e5 := argNum(args, 4)
	if bad := firstBad(e, e2, e3, e4, e5); bad.isError() {
		return bad
	}
	return numberValue(pmtValue(rate, nper, pv, fv, typ))
}

// pmtValue computes the payment (rate 0 is a plain division).
func pmtValue(rate, nper, pv, fv, typ floatVal) floatVal {
	if rate == 0 {
		return -(pv + fv) / nper
	}
	pow := growth(rate, nper)
	return -(pv*pow + fv) * rate / ((pow - 1) * (1 + rate*typ))
}

// fnFv is the future value of an annuity: FV(rate, nper, pmt, [pv], [type]).
func fnFv(args []Value) Value {
	rate, e := argNum(args, 0)
	nper, e2 := argNum(args, 1)
	pmt, e3 := argNum(args, 2)
	pv, e4 := argNum(args, 3)
	typ, e5 := argNum(args, 4)
	if bad := firstBad(e, e2, e3, e4, e5); bad.isError() {
		return bad
	}
	if rate == 0 {
		return numberValue(-(pv + pmt*nper))
	}
	pow := growth(rate, nper)
	return numberValue(-(pv*pow + pmt*(1+rate*typ)*(pow-1)/rate))
}

// fnPv is the present value of an annuity: PV(rate, nper, pmt, [fv], [type]).
func fnPv(args []Value) Value {
	rate, e := argNum(args, 0)
	nper, e2 := argNum(args, 1)
	pmt, e3 := argNum(args, 2)
	fv, e4 := argNum(args, 3)
	typ, e5 := argNum(args, 4)
	if bad := firstBad(e, e2, e3, e4, e5); bad.isError() {
		return bad
	}
	if rate == 0 {
		return numberValue(-(fv + pmt*nper))
	}
	pow := growth(rate, nper)
	return numberValue(-(fv + pmt*(1+rate*typ)*(pow-1)/rate) / pow)
}

// fnNpv is the net present value of a series at a discount rate: NPV(rate, v1, …).
func fnNpv(args []Value) Value {
	rate, bad := args[0].asNumber()
	if bad.isError() {
		return bad
	}
	total := 0.0
	for period, arg := range args[1:] {
		n, e := arg.asNumber()
		if e.isError() {
			return e
		}
		total += n / math.Pow(1+rate, float64(period+1))
	}
	return numberValue(floatVal(total))
}

// fnSln is straight-line depreciation: SLN(cost, salvage, life).
func fnSln(args []Value) Value {
	cost, e := argNum(args, 0)
	salvage, e2 := argNum(args, 1)
	life, e3 := argNum(args, 2)
	if bad := firstBad(e, e2, e3); bad.isError() {
		return bad
	}
	if life == 0 {
		return errorValue(ErrDiv)
	}
	return numberValue((cost - salvage) / life)
}

// firstBad returns the first error value among the given values.
func firstBad(values ...Value) Value {
	for _, v := range values {
		if v.isError() {
			return v
		}
	}
	return Value{}
}
