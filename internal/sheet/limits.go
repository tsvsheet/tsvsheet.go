package sheet

// Resource limits guard against out-of-memory from untrusted formula or edit
// input: no single cell may drive an unbounded array, string, or grid
// allocation. They are injected into every compute pass (ComputeOptions.Limits)
// and every edit (Sheet.Set), never held in a mutable global — so a frontend
// chooses its ceilings explicitly and concurrent passes never share state: the
// CLI keeps DefaultLimits (or honors --max-cells), the WASM build applies the
// smaller BrowserLimits.

// Limits bounds the sizes an untrusted sheet may drive an allocation to.
type Limits struct {
	ResultCells int // cells in one array formula result (e.g. SEQUENCE)
	GridDim     int // the highest row or column index the grid may grow to (Set)
	ResultBytes int // bytes in one string formula result (e.g. REPT)
}

// DefaultLimits are generous for real spreadsheets while still bounding OOM.
func DefaultLimits() Limits {
	return Limits{ResultCells: 5_000_000, GridDim: 1_000_000, ResultBytes: 1 << 20}
}

// BrowserLimits are the tighter ceilings the WASM build applies, sized for a
// browser tab rather than a workstation.
func BrowserLimits() Limits {
	return Limits{ResultCells: 100_000, GridDim: 20_000, ResultBytes: 64 << 10}
}

// resultDim is a row or column count of an array formula result.
type resultDim int

// tooManyCells reports whether a rows×cols array result exceeds the cell budget
// (computed in int64 so the product cannot overflow).
func (l Limits) tooManyCells(rows, cols resultDim) bool {
	return int64(rows)*int64(cols) > int64(l.ResultCells)
}

// effectiveLimits resolves the limits for a compute pass: the zero value (an
// unset ComputeOptions.Limits) falls back to DefaultLimits, any other value is
// honored verbatim.
func effectiveLimits(l Limits) Limits {
	if l == (Limits{}) {
		return DefaultLimits()
	}
	return l
}
