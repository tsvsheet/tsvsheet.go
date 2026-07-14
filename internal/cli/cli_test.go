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
)

const (
	sampleData     = "1\t2\t3\t4\n2\t3\t4\t5\n3\t4\t5\t6\n"
	sampleTemplate = "=header(1)\nA\tB\tC\tD\tE\n=body\nE=C + D\n"
)

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

func TestRunRender_TemplateStdinDataFile(t *testing.T) {
	t.Parallel()

	dataPath := writeTemp(t, "d.tsv", sampleData)
	streams, out, _ := streamsWith(sampleTemplate)
	err := runRender(streams, "-", sourcePath(dataPath))
	require.NoError(t, err)
	assert.Equal(t, "1\t2\t3\t4\t7\n2\t3\t4\t5\t9\n3\t4\t5\t6\t11\n", out.String())
}

func TestRunRender_BothStdin(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("")
	err := runRender(streams, "", "")
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrInvalidValue)
}

func TestRunRender_TemplateFileMissing(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith(sampleData)
	err := runRender(streams, "/no/such.tsvt", "-")
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrOpenFile)
}

func TestRunRender_DataFileMissing(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith(sampleTemplate)
	err := runRender(streams, "-", "/no/such.tsv")
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrOpenFile)
}

func TestRunRender_SyntaxError(t *testing.T) {
	t.Parallel()

	dataPath := writeTemp(t, "d.tsv", sampleData)
	streams, _, _ := streamsWith("=sum(")
	err := runRender(streams, "-", sourcePath(dataPath))
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrSyntax)
}

func TestRunRender_ComputeRejected(t *testing.T) {
	t.Parallel()

	dataPath := writeTemp(t, "d.tsv", sampleData)
	streams, _, _ := streamsWith("=final\n=A:C<") // range-scoped structural
	err := runRender(streams, "-", sourcePath(dataPath))
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrUnsupported)
}

func TestRunRender_WriteError(t *testing.T) {
	t.Parallel()

	dataPath := writeTemp(t, "d.tsv", sampleData)
	streams := Streams{In: strings.NewReader(sampleTemplate), Out: failWriter{}, Err: &bytes.Buffer{}}
	err := runRender(streams, "-", sourcePath(dataPath))
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrWriteFile)
}

func TestRunRender_TemplateFileReads(t *testing.T) {
	t.Parallel()

	tmplPath := writeTemp(t, "t.tsvt", sampleTemplate)
	streams, out, _ := streamsWith(sampleData)
	err := runRender(streams, sourcePath(tmplPath), "-")
	require.NoError(t, err)
	assert.Contains(t, out.String(), "\t7\n")
}

func TestRunCheck_Clean(t *testing.T) {
	t.Parallel()

	streams, _, errBuf := streamsWith("=body\nE=C + D\n")
	require.NoError(t, runCheck(streams, "-"))
	assert.Empty(t, errBuf.String())
}

func TestRunCheck_Diagnostics(t *testing.T) {
	t.Parallel()

	streams, _, errBuf := streamsWith("=body\nC!\n")
	err := runCheck(streams, "-")
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrDiagnostics)
	assert.Contains(t, errBuf.String(), "line 2")
}

func TestRunCheck_SyntaxError(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("=sum(")
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

	dataPath := writeTemp(t, "d.tsv", sampleData)
	streams, out, _ := streamsWith("=body\nE=C + D\n")
	err := runExplain(streams, explainConfig{template: "-", data: sourcePath(dataPath), cell: "E2"})
	require.NoError(t, err)
	assert.Contains(t, out.String(), "E2 = 9")
	assert.Contains(t, out.String(), "formula: C + D")
}

func TestRunExplain_JSON(t *testing.T) {
	t.Parallel()

	dataPath := writeTemp(t, "d.tsv", sampleData)
	streams, out, _ := streamsWith("=body\nE=C + D\n")
	err := runExplain(streams, explainConfig{template: "-", data: sourcePath(dataPath), cell: "E2", isJSON: true})
	require.NoError(t, err)
	assert.Contains(t, out.String(), `"cell": "E2"`)
	assert.Contains(t, out.String(), `"formula": "C + D"`)
}

func TestRunExplain_PlainCellNoFormula(t *testing.T) {
	t.Parallel()

	dataPath := writeTemp(t, "d.tsv", sampleData)
	streams, out, _ := streamsWith("=body\nE=C\n")
	err := runExplain(streams, explainConfig{template: "-", data: sourcePath(dataPath), cell: "A1", isJSON: true})
	require.NoError(t, err)
	assert.Contains(t, out.String(), `"value": "1"`)
}

func TestRunExplain_BadCell(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("=body\nE=C\n")
	err := runExplain(streams, explainConfig{template: "-", data: "-", cell: "bogus"})
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrInvalidValue)
}

func TestRunExplain_BothStdin(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("")
	err := runExplain(streams, explainConfig{cell: "A1"})
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrInvalidValue)
}

func TestRunExplain_TemplateSyntaxError(t *testing.T) {
	t.Parallel()

	dataPath := writeTemp(t, "d.tsv", sampleData)
	streams, _, _ := streamsWith("=sum(")
	err := runExplain(streams, explainConfig{template: "-", data: sourcePath(dataPath), cell: "A1"})
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrSyntax)
}

func TestRunExplain_OutOfGrid(t *testing.T) {
	t.Parallel()

	dataPath := writeTemp(t, "d.tsv", sampleData)
	streams, _, _ := streamsWith("=body\nE=C\n")
	err := runExplain(streams, explainConfig{template: "-", data: sourcePath(dataPath), cell: "Z99"})
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrNotFound)
}

func TestRunExplain_WriteError(t *testing.T) {
	t.Parallel()

	dataPath := writeTemp(t, "d.tsv", sampleData)
	streams := Streams{In: strings.NewReader("=body\nE=C\n"), Out: failWriter{}, Err: &bytes.Buffer{}}
	err := runExplain(streams, explainConfig{template: "-", data: sourcePath(dataPath), cell: "A1", isJSON: true})
	require.Error(t, err)
}

func TestRunParse_JSON(t *testing.T) {
	t.Parallel()

	streams, out, _ := streamsWith("=header(1)\nA\tB\n=body\nE=C + D\n")
	require.NoError(t, runParse(streams, "-"))
	assert.Contains(t, out.String(), `"kind": "header"`)
	assert.Contains(t, out.String(), `"kind": "row"`)
	assert.Contains(t, out.String(), `"kind": "body"`)
	assert.Contains(t, out.String(), `"source": "E=C + D"`)
}

func TestRunParse_Structural(t *testing.T) {
	t.Parallel()

	streams, out, _ := streamsWith("=final\n=A<\n")
	require.NoError(t, runParse(streams, "-"))
	assert.Contains(t, out.String(), `"kind": "structural"`)
	assert.Contains(t, out.String(), `"kind": "final"`)
}

func TestRunParse_SyntaxError(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("=sum(")
	err := runParse(streams, "-")
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrSyntax)
}

func TestRunParse_FileMissing(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("")
	err := runParse(streams, "/no/such.tsvt")
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrOpenFile)
}
