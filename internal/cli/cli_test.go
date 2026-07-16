package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/constants"
	"github.com/uplang/tsvsheet.go/internal/sheet"
)

// sampleSheet is a single-file spreadsheet: two data columns and a C-column
// formula summing A and B per row.
const sampleSheet = "2\t3\t=A1+B1\n4\t5\t=A2+B2\n"

// streamsWith builds Streams over the given input, capturing out and err.
func streamsWith(in string) (Streams, *bytes.Buffer, *bytes.Buffer) {
	var out, errBuf bytes.Buffer
	return Streams{In: strings.NewReader(in), Out: &out, Err: &errBuf}, &out, &errBuf
}

// writeTemp writes content to a temp file and returns its path.
func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// failWriter always fails, exercising output error paths.
type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestRunRender_ComputesFromStdin(t *testing.T) {
	t.Parallel()

	streams, out, _ := streamsWith(sampleSheet)
	require.NoError(t, runRender(streams, "-", false, sheet.DefaultLimits()))
	assert.Equal(t, "2\t3\t5\n4\t5\t9\n", out.String())
}

func TestRunRender_ReadsFile(t *testing.T) {
	t.Parallel()

	path := writeTemp(t, "s.tsvt", sampleSheet)
	streams, out, _ := streamsWith("")
	require.NoError(t, runRender(streams, sourcePath(path), false, sheet.DefaultLimits()))
	assert.Contains(t, out.String(), "\t5\n")
}

func TestRunRender_FileMissing(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("")
	err := runRender(streams, "/no/such.tsvt", false, sheet.DefaultLimits())
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrOpenFile)
}

func TestRunRender_SyntaxError(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("1\t=sum(\n")
	err := runRender(streams, "-", false, sheet.DefaultLimits())
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrSyntax)
}

func TestRunRender_WriteError(t *testing.T) {
	t.Parallel()

	streams := Streams{In: strings.NewReader(sampleSheet), Out: failWriter{}, Err: &bytes.Buffer{}}
	err := runRender(streams, "-", false, sheet.DefaultLimits())
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrWriteFile)
}

func TestRunCheck_Clean(t *testing.T) {
	t.Parallel()

	streams, _, errBuf := streamsWith(sampleSheet)
	require.NoError(t, runCheck(streams, "-"))
	assert.Empty(t, errBuf.String())
}

func TestRunCheck_Diagnostics(t *testing.T) {
	t.Parallel()

	streams, _, errBuf := streamsWith("=bogus(A1)\n")
	err := runCheck(streams, "-")
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrDiagnostics)
	assert.Contains(t, errBuf.String(), "A1: unknown function: bogus")
}

func TestRunCheck_SyntaxError(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("1\t=sum(\n")
	err := runCheck(streams, "-")
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrSyntax)
}

func TestRunCheck_FileMissing(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("")
	err := runCheck(streams, "/no/such.tsvt")
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrOpenFile)
}

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
	assert.ErrorIs(t, err, constants.ErrInvalidValue)
}

func TestRunExplain_SyntaxError(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("1\t=sum(\n")
	err := runExplain(streams, explainConfig{source: "-", cell: "A1"})
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrSyntax)
}

func TestRunExplain_OutOfGrid(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith(sampleSheet)
	err := runExplain(streams, explainConfig{source: "-", cell: "Z99"})
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrNotFound)
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

func TestRunParse_JSON(t *testing.T) {
	t.Parallel()

	// The source grid is emitted verbatim as "rows"; without --value there is
	// no "values".
	streams, out, _ := streamsWith("a\t\t=A1\n")
	require.NoError(t, runParse(streams, "-", false, false, sheet.DefaultLimits()))
	body := out.String()
	assert.Contains(t, body, `"rows"`)
	assert.Contains(t, body, `"=A1"`)
	assert.NotContains(t, body, `"values"`)
}

func TestRunParse_WithValues(t *testing.T) {
	t.Parallel()

	// --value adds the computed grid.
	streams, out, _ := streamsWith("2\t=A1*10\n")
	require.NoError(t, runParse(streams, "-", true, false, sheet.DefaultLimits()))
	body := out.String()
	assert.Contains(t, body, `"values"`)
	assert.Contains(t, body, `"20"`) // A1*10 = 20
}

func TestRunFromJSON_RoundTrip(t *testing.T) {
	t.Parallel()

	streams, out, _ := streamsWith(`{"rows":[["a","","=A1"],["1","2","3"]]}`)
	require.NoError(t, runFromJSON(streams, "-"))
	assert.Equal(t, "a\t\t=A1\n1\t2\t3\n", out.String())
}

func TestRunFromJSON_BadJSON(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("not json")
	err := runFromJSON(streams, "-")
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrSyntax)
}

func TestRunFromJSON_FileMissing(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("")
	err := runFromJSON(streams, "/no/such.json")
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrOpenFile)
}

func TestRunParse_SyntaxError(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("1\t=sum(\n")
	err := runParse(streams, "-", false, false, sheet.DefaultLimits())
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrSyntax)
}

func TestRunParse_FileMissing(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("")
	err := runParse(streams, "/no/such.tsvt", false, false, sheet.DefaultLimits())
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrOpenFile)
}
