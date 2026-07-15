package tsvt

import "testing"

// TestSeal exercises the sealed-interface marker methods (empty bodies that
// bound each interface's variant set at compile time; nothing else calls them).
func TestSeal(t *testing.T) {
	t.Parallel()
	Number{}.isExpr()
	RangeRef{}.isReference()
}
