package sheet_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasImports(t *testing.T) {
	t.Parallel()

	assert.True(t, parse(t, "=importcell(\"https://x.example/v\")\n").HasImports())
	assert.True(t, parse(t, "=importsheet(\"https://x.example/v\")\n").HasImports())
	assert.False(t, parse(t, "=sum(A1:A2)\n").HasImports()) // a call, but not an import
	assert.False(t, parse(t, "plain\n").HasImports())       // no formula at all
}

func TestImportDisabledYieldsImportError(t *testing.T) {
	t.Parallel()

	for _, src := range []string{
		"=importcell(\"https://x.example/v\")\n",
		"=importrow(\"https://x.example/v\")\n",
		"=importcolumn(\"https://x.example/v\")\n",
		"=importrange(\"https://x.example/v\")\n",
		"=importsheet(\"https://x.example/v\")\n",
	} {
		assert.Equal(t, "#IMPORT!", cellAt(t, compute(t, src), 0, 0), "no Fetcher injected: %s must be #IMPORT!", src)
	}
}

func TestImportErrorLiteralPropagates(t *testing.T) {
	t.Parallel()

	// A cell literally holding #IMPORT! round-trips as an error value and
	// propagates through a reference (isErrorCode recognizes it).
	assert.Equal(t, "#IMPORT!", cellAt(t, compute(t, "#IMPORT!\t=A1\n"), 0, 1))
}
