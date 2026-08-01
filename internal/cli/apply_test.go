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
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

// writeSheet writes content as a .tsvt file in a fresh temp dir and returns
// its path.
func writeSheet(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sheet.tsvt")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// writeEdits writes content as an edits file next to nothing in particular.
func writeEdits(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "batch.edits")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// revisionOf is the content address the engine assigns to src.
func revisionOf(t *testing.T, src string) string {
	t.Helper()
	doc, err := tsvsheet.ParseDocument([]byte(src))
	require.NoError(t, err)
	return string(tsvsheet.Revision(doc))
}

func TestRunApply_EditsFileToSheetFile(t *testing.T) {
	t.Parallel()

	sheet := writeSheet(t, "1\t2\n")
	edits := writeEdits(t, "setCell\tB1\t9\n")
	streams, outBuf, _ := streamsWith("")
	require.NoError(t, runApply(streams, sourcePath(sheet), sourcePath(edits), tsvsheet.DefaultLimits(), applyWrite))
	saved, err := os.ReadFile(sheet)
	require.NoError(t, err)
	assert.Equal(t, "1\t9\n", string(saved))
	assert.Equal(t, revisionOf(t, "1\t9\n")+"\n", outBuf.String())
}

func TestRunApply_EditsFromStdin(t *testing.T) {
	t.Parallel()

	sheet := writeSheet(t, "1\t2\n")
	streams, _, _ := streamsWith("setCell\tA1\t7\n")
	require.NoError(t, runApply(streams, sourcePath(sheet), "-", tsvsheet.DefaultLimits(), applyWrite))
	saved, err := os.ReadFile(sheet)
	require.NoError(t, err)
	assert.Equal(t, "7\t2\n", string(saved))
}

func TestRunApply_SheetStdinWritesResultToStdout(t *testing.T) {
	t.Parallel()

	edits := writeEdits(t, "setCell\tB1\t9\n")
	streams, outBuf, errBuf := streamsWith("1\t2\n")
	require.NoError(t, runApply(streams, "-", sourcePath(edits), tsvsheet.DefaultLimits(), applyWrite))
	assert.Equal(t, "1\t9\n", outBuf.String())
	assert.Contains(t, errBuf.String(), revisionOf(t, "1\t9\n"))
}

func TestRunApply_BothStdinRefused(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("")
	err := runApply(streams, "-", "-", tsvsheet.DefaultLimits(), applyWrite)
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrApplyBothStdin)
}

func TestRunApply_BaseMismatchRefusesAndLeavesFile(t *testing.T) {
	t.Parallel()

	sheet := writeSheet(t, "1\t2\n")
	edits := writeEdits(t, "#.base\t"+strings.Repeat("0", 64)+"\nsetCell\tB1\t9\n")
	streams, _, _ := streamsWith("")
	err := runApply(streams, sourcePath(sheet), sourcePath(edits), tsvsheet.DefaultLimits(), applyWrite)
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrEditsBase)
	saved, readErr := os.ReadFile(sheet)
	require.NoError(t, readErr)
	assert.Equal(t, "1\t2\n", string(saved))
}

func TestRunApply_BaseMatchApplies(t *testing.T) {
	t.Parallel()

	sheet := writeSheet(t, "1\t2\n")
	edits := writeEdits(t, "#.base\t"+revisionOf(t, "1\t2\n")+"\nsetCell\tB1\t9\n")
	streams, _, errBuf := streamsWith("")
	require.NoError(t, runApply(streams, sourcePath(sheet), sourcePath(edits), tsvsheet.DefaultLimits(), applyWrite))
	assert.NotContains(t, errBuf.String(), noBaseNotice)
}

func TestRunApply_NoBaseNoticeOnStderr(t *testing.T) {
	t.Parallel()

	sheet := writeSheet(t, "1\t2\n")
	edits := writeEdits(t, "setCell\tB1\t9\n")
	streams, _, errBuf := streamsWith("")
	require.NoError(t, runApply(streams, sourcePath(sheet), sourcePath(edits), tsvsheet.DefaultLimits(), applyWrite))
	assert.Contains(t, errBuf.String(), noBaseNotice)
}

func TestRunApply_CheckDryRunLeavesFileAndPrintsRevision(t *testing.T) {
	t.Parallel()

	sheet := writeSheet(t, "1\t2\n")
	edits := writeEdits(t, "setCell\tB1\t9\n")
	streams, outBuf, _ := streamsWith("")
	require.NoError(
		t,
		runApply(streams, sourcePath(sheet), sourcePath(edits), tsvsheet.DefaultLimits(), applyCheckOnly),
	)
	saved, err := os.ReadFile(sheet)
	require.NoError(t, err)
	assert.Equal(t, "1\t2\n", string(saved))
	assert.Equal(t, revisionOf(t, "1\t9\n")+"\n", outBuf.String())
}

func TestRunApply_CheckDryRunStillRefusesBadBase(t *testing.T) {
	t.Parallel()

	sheet := writeSheet(t, "1\t2\n")
	edits := writeEdits(t, "#.base\t"+strings.Repeat("0", 64)+"\nsetCell\tB1\t9\n")
	streams, _, _ := streamsWith("")
	err := runApply(streams, sourcePath(sheet), sourcePath(edits), tsvsheet.DefaultLimits(), applyCheckOnly)
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrEditsBase)
}

func TestRunApply_MalformedEditsRefused(t *testing.T) {
	t.Parallel()

	sheet := writeSheet(t, "1\t2\n")
	edits := writeEdits(t, "nope\tA1\n")
	streams, _, _ := streamsWith("")
	err := runApply(streams, sourcePath(sheet), sourcePath(edits), tsvsheet.DefaultLimits(), applyWrite)
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrEditsOp)
}

func TestRunApply_MissingSheetFile(t *testing.T) {
	t.Parallel()

	edits := writeEdits(t, "setCell\tA1\tx\n")
	streams, _, _ := streamsWith("")
	err := runApply(streams, "/no/such.tsvt", sourcePath(edits), tsvsheet.DefaultLimits(), applyWrite)
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrOpenFile)
}

func TestRunApply_MissingEditsFile(t *testing.T) {
	t.Parallel()

	sheet := writeSheet(t, "1\t2\n")
	streams, _, _ := streamsWith("")
	err := runApply(streams, sourcePath(sheet), "/no/such.edits", tsvsheet.DefaultLimits(), applyWrite)
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrOpenFile)
}

func TestRunApply_MalformedSheetRefused(t *testing.T) {
	t.Parallel()

	sheet := writeSheet(t, "=(\n")
	edits := writeEdits(t, "setCell\tA1\tx\n")
	streams, _, _ := streamsWith("")
	err := runApply(streams, sourcePath(sheet), sourcePath(edits), tsvsheet.DefaultLimits(), applyWrite)
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrSyntax)
}

// brokenReader always fails, driving the edits read-error path.
type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, errors.New("broken pipe") }

func TestRunApply_EditsReadFailure(t *testing.T) {
	t.Parallel()

	sheet := writeSheet(t, "1\t2\n")
	streams := Streams{In: brokenReader{}, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := runApply(streams, sourcePath(sheet), "-", tsvsheet.DefaultLimits(), applyWrite)
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrReadInput)
}

func TestRunApply_WriteFailureLeavesSheetIntact(t *testing.T) {
	// Serial: swaps the process-global writeFileIn stub.
	sheet := writeSheet(t, "1\t2\n")
	edits := writeEdits(t, "setCell\tB1\t9\n")
	prev := writeFileIn
	writeFileIn = func(*os.Root, string, []byte, os.FileMode) error { return errors.New("disk full") }
	t.Cleanup(func() { writeFileIn = prev })
	streams, _, _ := streamsWith("")
	err := runApply(streams, sourcePath(sheet), sourcePath(edits), tsvsheet.DefaultLimits(), applyWrite)
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrWriteFile)
	saved, readErr := os.ReadFile(sheet)
	require.NoError(t, readErr)
	assert.Equal(t, "1\t2\n", string(saved))
}

func TestRunApply_BareFilenameSavesInCurrentDirectory(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bare.tsvt"), []byte("1\t2\n"), 0o600))
	t.Chdir(dir)
	edits := writeEdits(t, "setCell\tB1\t9\n")
	streams, _, _ := streamsWith("")
	require.NoError(t, runApply(streams, "bare.tsvt", sourcePath(edits), tsvsheet.DefaultLimits(), applyWrite))
	saved, err := os.ReadFile(filepath.Join(dir, "bare.tsvt"))
	require.NoError(t, err)
	assert.Equal(t, "1\t9\n", string(saved))
}

func TestCLI_Apply(t *testing.T) {
	sheet := writeSheet(t, "1\t2\n")
	edits := writeEdits(t, "setCell\tB1\t9\n")
	_, err := runCLI(t, cmdApply, sheet, edits)
	require.NoError(t, err)
	saved, readErr := os.ReadFile(sheet)
	require.NoError(t, readErr)
	assert.Equal(t, "1\t9\n", string(saved))
}

func TestCLI_ApplyCheckFlag(t *testing.T) {
	sheet := writeSheet(t, "1\t2\n")
	edits := writeEdits(t, "setCell\tB1\t9\n")
	_, err := runCLI(t, cmdApply, "--check", sheet, edits)
	require.NoError(t, err)
	saved, readErr := os.ReadFile(sheet)
	require.NoError(t, readErr)
	assert.Equal(t, "1\t2\n", string(saved))
}

func TestCLI_ApplyEditsOnStdin(t *testing.T) {
	sheet := writeSheet(t, "1\t2\n")
	withStdin(t, "setCell\tA1\t7\n")
	_, err := runCLI(t, cmdApply, sheet)
	require.NoError(t, err)
	saved, readErr := os.ReadFile(sheet)
	require.NoError(t, readErr)
	assert.Equal(t, "7\t2\n", string(saved))
}
