package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

func TestRunParse_JSON(t *testing.T) {
	t.Parallel()

	// The source grid is emitted verbatim as "rows"; without --value there is
	// no "values".
	streams, out, _ := streamsWith("a\t\t=A1\n")
	require.NoError(t, runParse(streams, "-", false, false, tsvsheet.DefaultLimits(), nil))
	body := out.String()
	assert.Contains(t, body, `"rows"`)
	assert.Contains(t, body, `"=A1"`)
	assert.NotContains(t, body, `"values"`)
}

func TestRunParse_WithValues(t *testing.T) {
	t.Parallel()

	// --value adds the computed grid.
	streams, out, _ := streamsWith("2\t=A1*10\n")
	require.NoError(t, runParse(streams, "-", true, false, tsvsheet.DefaultLimits(), nil))
	body := out.String()
	assert.Contains(t, body, `"values"`)
	assert.Contains(t, body, `"20"`) // A1*10 = 20
}

func TestRunFromJSON_RoundTrip(t *testing.T) {
	t.Parallel()

	streams, out, _ := streamsWith(`{"rows":[["a","","=A1"],["1","2","3"]]}`)
	require.NoError(t, runFromJSON(streams, "-", tsvsheet.DefaultLimits()))
	assert.Equal(t, "a\t\t=A1\n1\t2\t3\n", out.String())
}

func TestRunFromJSON_BadJSON(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("not json")
	err := runFromJSON(streams, "-", tsvsheet.DefaultLimits())
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrSyntax)
}

func TestRunFromJSON_FileMissing(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("")
	err := runFromJSON(streams, "/no/such.json", tsvsheet.DefaultLimits())
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrOpenFile)
}

func TestRunParse_SyntaxError(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("1\t=sum(\n")
	err := runParse(streams, "-", false, false, tsvsheet.DefaultLimits(), nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrSyntax)
}

func TestRunParse_FileMissing(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("")
	err := runParse(streams, "/no/such.tsvt", false, false, tsvsheet.DefaultLimits(), nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrOpenFile)
}

func TestCLI_Parse(t *testing.T) {
	withStdin(t, sampleSheet)
	out, err := runCLI(t, "parse")
	require.NoError(t, err)
	assert.Contains(t, out, `"rows"`)
	assert.Contains(t, out, `=A1+B1`)      // source grid carries the formula
	assert.NotContains(t, out, `"values"`) // computed grid omitted without the flag
}

func TestCLI_ParseWithValue(t *testing.T) {
	withStdin(t, sampleSheet)
	out, err := runCLI(t, "parse", "--value")
	require.NoError(t, err)
	assert.Contains(t, out, `"values"`)
	assert.Contains(t, out, `"5"`) // C1 = A1+B1 = 5, in the computed grid
}

func TestCLI_ParseRoundTripsThroughFromJSON(t *testing.T) {
	// parse → from-json reconstructs the original source.
	json, err := runCLI(t, "parse", writeTemp(t, "s.tsvt", sampleSheet))
	require.NoError(t, err)
	withStdin(t, json)
	back, err := runCLI(t, "from-json")
	require.NoError(t, err)
	assert.Equal(t, sampleSheet, back)
}

func TestRunParse_ReadError(t *testing.T) {
	t.Parallel()

	streams := Streams{In: failReader{}, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := runParse(streams, "-", false, false, tsvsheet.DefaultLimits(), nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrReadInput)
}

// TestRunFromJSON_RefusesOverBudgetGrids pins the JSON round trip's budget at
// the unit level (the integration suite drives it through the full CLI): a
// decoded grid over the resident ceiling refuses as ErrDocTooLarge with the
// flag hint; in budget it round-trips.
func TestRunFromJSON_RefusesOverBudgetGrids(t *testing.T) {
	t.Parallel()

	tight := tsvsheet.DefaultLimits()
	tight.ResidentCells = 2
	path := writeTemp(t, "grid.json", `{"rows":[["a","b"],["c","d"]]}`)
	err := runFromJSON(Streams{Out: &bytes.Buffer{}}, sourcePath(path), tight)
	require.ErrorIs(t, err, tsvsheet.ErrDocTooLarge)
	assert.Contains(t, err.Error(), "--max-cells")

	out := &bytes.Buffer{}
	require.NoError(t, runFromJSON(Streams{Out: out}, sourcePath(path), tsvsheet.DefaultLimits()))
	assert.Equal(t, "a\tb\nc\td\n", out.String())
}
