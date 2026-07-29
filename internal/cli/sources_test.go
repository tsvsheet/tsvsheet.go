package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNoClose_LeavesStdinOpen pins why the stdin release is a no-op rather than
// a Close: a command that closed os.Stdin would break the next stage of a
// pipeline, and every caller defers the release unconditionally — the no-op is
// what makes that unconditional defer safe.
func TestNoClose_LeavesStdinOpen(t *testing.T) {
	t.Parallel()

	assert.NoError(t, noClose(), "the stdin release never reports failure")

	in := strings.NewReader("a\tb\n")
	reader, release, err := sourcePath("-").open(in)
	require.NoError(t, err)
	require.NoError(t, release(), "releasing stdin is a no-op")

	// Still readable after the release; a real Close would have broken this.
	rest, err := readAll(reader)
	require.NoError(t, err)
	assert.Equal(t, "a\tb\n", string(rest))
}

// TestSourcePath_EmptyAndDashBothMeanStdin pins the two spellings a command
// accepts for "read standard input", so an omitted positional and an explicit
// "-" cannot diverge.
func TestSourcePath_EmptyAndDashBothMeanStdin(t *testing.T) {
	t.Parallel()

	assert.True(t, sourcePath("").isStdin())
	assert.True(t, sourcePath("-").isStdin())
	assert.False(t, sourcePath("sheet.tsvt").isStdin())
}
