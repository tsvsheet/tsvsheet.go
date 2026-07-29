package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

func TestRunExplain_Text(t *testing.T) {
	t.Parallel()

	streams, out, _ := streamsWith(sampleSheet)
	err := runExplain(streams, explainConfig{source: "-", cell: "C1"})
	require.NoError(t, err)
	assert.Contains(t, out.String(), "C1 = 5")
	assert.Contains(t, out.String(), "formula: A1 + B1")
}

func TestRunExplain_JSON(t *testing.T) {
	t.Parallel()

	streams, out, _ := streamsWith(sampleSheet)
	err := runExplain(streams, explainConfig{source: "-", cell: "C1", isJSON: true})
	require.NoError(t, err)
	assert.Contains(t, out.String(), `"cell": "C1"`)
	assert.Contains(t, out.String(), `"formula": "A1 + B1"`)
}

func TestRunExplain_LiteralCellNoFormula(t *testing.T) {
	t.Parallel()

	streams, out, _ := streamsWith(sampleSheet)
	err := runExplain(streams, explainConfig{source: "-", cell: "A1", isJSON: true})
	require.NoError(t, err)
	assert.Contains(t, out.String(), `"value": "2"`)
}

func TestRunExplain_BadCell(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith(sampleSheet)
	err := runExplain(streams, explainConfig{source: "-", cell: "bogus"})
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrInvalidValue)
}

func TestRunExplain_SyntaxError(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("1\t=sum(\n")
	err := runExplain(streams, explainConfig{source: "-", cell: "A1"})
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrSyntax)
}

func TestRunExplain_OutOfGrid(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith(sampleSheet)
	err := runExplain(streams, explainConfig{source: "-", cell: "Z99"})
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrNotFound)
}

func TestRunExplain_WriteError(t *testing.T) {
	t.Parallel()

	streams := Streams{In: strings.NewReader(sampleSheet), Out: failWriter{}, Err: &bytes.Buffer{}}
	err := runExplain(streams, explainConfig{source: "-", cell: "A1", isJSON: true})
	require.Error(t, err)
}

func TestRunExplain_FileMissing(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("")
	err := runExplain(streams, explainConfig{source: "/no/such.tsvt", cell: "A1"})
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrOpenFile)
}

func TestCLI_ExplainCell(t *testing.T) {
	path := writeTemp(t, "s.tsvt", sampleSheet)
	out, err := runCLI(t, "explain", "C1", path)
	require.NoError(t, err)
	assert.Contains(t, out, "C1 = 5")
}

func TestRunExplain_ReadError(t *testing.T) {
	t.Parallel()

	streams := Streams{In: failReader{}, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := runExplain(streams, explainConfig{source: "-", cell: "A1"})
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrReadInput)
}
