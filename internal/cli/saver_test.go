package cli

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"
)

// TestSaveAtomic_ConcurrentSavesDoNotShareStaging pins the fix for the
// fixed-staging-name race: two concurrent saves of one sheet must each publish
// their own bytes, never one another's, and never report a failure for a write
// that landed.
func TestSaveAtomic_ConcurrentSavesDoNotShareStaging(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "race.tsvt"), []byte("0\n"), 0o600))
	payloads := []string{strings.Repeat("a\n", 20000), strings.Repeat("b\n", 20000)}
	var wg sync.WaitGroup
	errs := make([]error, len(payloads))
	for i, payload := range payloads {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = saveAtomic(sheetDir(dir), "race.tsvt", []byte(payload))
		}()
	}
	wg.Wait()
	for i, err := range errs {
		require.NoError(t, err, "save %d reported a failure", i)
	}
	saved, err := os.ReadFile(filepath.Join(dir, "race.tsvt"))
	require.NoError(t, err)
	assert.Contains(t, payloads, string(saved), "the sheet holds one writer's bytes, whole")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no staging file survives")
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

// TestCLI_ApplyStdinSheetThroughEndOfFlags pins the documented stdin-sheet
// invocation end to end: a bare "-" swallows the positionals behind it, so the
// form the help prints is the "--" form, and it must work.
func TestCLI_ApplyStdinSheetThroughEndOfFlags(t *testing.T) {
	edits := writeEdits(t, "setCell\tB1\t9\n")
	withStdin(t, "1\t2\n")
	out, err := runCLI(t, cmdApply, "--", "-", edits)
	require.NoError(t, err)
	assert.Equal(t, "1\t9\n", out)
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
