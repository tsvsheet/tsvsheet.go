package sheet

import (
	"math"
	"sort"
)

// meanOf is the arithmetic mean of a non-empty slice.
func meanOf(nums []float64) float64 {
	total := 0.0
	for _, n := range nums {
		total += n
	}
	return total / float64(len(nums))
}

// fnMedian is the middle value (mean of the two middle values for an even
// count); an empty set is #NUM!.
func fnMedian(args []Value) Value {
	nums, bad, ok := numerics(args)
	if !ok {
		return bad
	}
	if len(nums) == 0 {
		return errorValue(ErrNum)
	}
	sort.Float64s(nums)
	mid := len(nums) / 2
	if len(nums)%2 == 1 {
		return numberValue(floatVal(nums[mid]))
	}
	return numberValue(floatVal((nums[mid-1] + nums[mid]) / 2))
}

// fnMode is the most frequent value; no repeat is #N/A.
func fnMode(args []Value) Value {
	nums, bad, ok := numerics(args)
	if !ok {
		return bad
	}
	counts := make(map[float64]int, len(nums))
	best, bestCount := 0.0, 1
	for _, n := range nums {
		counts[n]++
		if counts[n] > bestCount {
			best, bestCount = n, counts[n]
		}
	}
	if bestCount < 2 {
		return errorValue(ErrNA)
	}
	return numberValue(floatVal(best))
}

// spread computes a standard deviation or variance; isSample selects the n-1
// denominator, isStdev takes the square root.
func spread(args []Value, isSample, isStdev boolResult) Value {
	nums, bad, ok := numerics(args)
	if !ok {
		return bad
	}
	denom := len(nums)
	if isSample {
		denom--
	}
	if denom < 1 {
		return errorValue(ErrDiv)
	}
	mean := meanOf(nums)
	sumSquares := 0.0
	for _, n := range nums {
		sumSquares += (n - mean) * (n - mean)
	}
	result := sumSquares / float64(denom)
	if isStdev {
		result = math.Sqrt(result)
	}
	return numberValue(floatVal(result))
}

func fnStdev(args []Value) Value  { return spread(args, true, true) }
func fnStdevp(args []Value) Value { return spread(args, false, true) }
func fnVar(args []Value) Value    { return spread(args, true, false) }
func fnVarp(args []Value) Value   { return spread(args, false, false) }

// fnGeomean is the geometric mean; a non-positive operand is #NUM!.
func fnGeomean(args []Value) Value {
	nums, bad, ok := numerics(args)
	if !ok {
		return bad
	}
	if len(nums) == 0 {
		return errorValue(ErrNum)
	}
	logSum := 0.0
	for _, n := range nums {
		if n <= 0 {
			return errorValue(ErrNum)
		}
		logSum += math.Log(n)
	}
	return numberValue(floatVal(math.Exp(logSum / float64(len(nums)))))
}

// fnLarge and fnSmall return the k-th largest/smallest value; the last argument
// is k. An out-of-range k is #NUM!.
func fnLarge(args []Value) Value { return rankPick(args, true) }
func fnSmall(args []Value) Value { return rankPick(args, false) }

func rankPick(args []Value, isLargest boolResult) Value {
	k, bad := intArg(args[len(args)-1])
	if bad.isError() {
		return bad
	}
	nums, nbad, ok := numerics(args[:len(args)-1])
	if !ok {
		return nbad
	}
	if k < 1 || int(k) > len(nums) {
		return errorValue(ErrNum)
	}
	sort.Float64s(nums)
	if isLargest {
		return numberValue(floatVal(nums[len(nums)-int(k)]))
	}
	return numberValue(floatVal(nums[k-1]))
}

// fnCountblank counts the empty operands.
func fnCountblank(args []Value) Value {
	count := 0
	for _, arg := range args {
		if arg.kind == kindEmpty {
			count++
		}
	}
	return numberValue(floatVal(count))
}
