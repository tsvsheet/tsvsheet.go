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

func TestExplain_ReportsWhereAnImportWent(t *testing.T) {
	t.Parallel()

	sheet := writeTemp(t, "imp.tsvt", "=importcell(\"balances.tsv\")\n")
	out, err := runCLI(t, "explain", "A1", "--data", dataDir(t), sheet)
	require.NoError(t, err)
	assert.Contains(t, out, "import balances.tsv -> http://127.0.0.1:")
	assert.Contains(t, out, "/balances.tsv")
}

func TestExplain_ReportsWhyAnImportFailed(t *testing.T) {
	t.Parallel()

	sheet := writeTemp(t, "bad.tsvt", "=importcell(\"nope.tsv\")\n")
	out, err := runCLI(t, "explain", "A1", "--data", dataDir(t), sheet)
	require.NoError(t, err)
	assert.Contains(t, out, "A1 = #IMPORT!")
	assert.Contains(t, out, "FAILED: ")
}

func TestExplain_JSONCarriesTheImportTrace(t *testing.T) {
	t.Parallel()

	sheet := writeTemp(t, "impj.tsvt", "=importcell(\"balances.tsv\")\n")
	out, err := runCLI(t, "explain", "A1", "--json", "--data", dataDir(t), sheet)
	require.NoError(t, err)
	assert.Contains(t, out, `"imports"`)
	assert.Contains(t, out, `"source": "balances.tsv"`)
}

func TestExplain_WithoutImportsReportsNoImportLines(t *testing.T) {
	t.Parallel()

	sheet := writeTemp(t, "plain.tsvt", "2\t3\t=A1+B1\n")
	out, err := runCLI(t, "explain", "C1", sheet)
	require.NoError(t, err)
	assert.Contains(t, out, "C1 = 5")
	assert.NotContains(t, out, "import ")
}

func TestExplain_BadDataFlagFailsBeforeTracing(t *testing.T) {
	t.Parallel()

	sheet := writeTemp(t, "e.tsvt", "1\n")
	_, err := runCLI(t, "explain", "A1", "--data", "http://data.example.com/", sheet)
	require.ErrorIs(t, err, constants.ErrImportScheme)
}

func TestExplain_BadImportFlagsFailBeforeTracing(t *testing.T) {
	t.Parallel()

	sheet := writeTemp(t, "ei.tsvt", "1\n")
	_, err := runCLI(t, "explain", "A1", "--allow-import", sheet)
	require.Error(t, err, "--allow-import with no --import-host is a configuration error")
}

func TestExplain_NotesWhereTheCellReadsDifferentlyThanInASpreadsheet(t *testing.T) {
	t.Parallel()

	sheet := writeTemp(t, "div.tsvt", "=-2^2\n")
	out, err := runCLI(t, "explain", "A1", sheet)
	require.NoError(t, err)

	// The value, and why it is that value rather than the 4 a spreadsheet user
	// expects. check says this at authoring time; explain has to say it at read
	// time too, or the lesson only reaches whoever ran the checker.
	assert.Contains(t, out, "A1 = -4")
	assert.Contains(t, out, "note: unary sign binds looser than ^")
	assert.Contains(t, out, "Excel reads (-x)^y")
}

func TestExplain_AddsNoNoteToACellThatReadsAlike(t *testing.T) {
	t.Parallel()

	sheet := writeTemp(t, "plain.tsvt", "=(1+2)*3\n")
	out, err := runCLI(t, "explain", "A1", sheet)
	require.NoError(t, err)

	assert.Contains(t, out, "A1 = 9")
	assert.NotContains(t, out, "note:")
}
