package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

// TestServeSheetConfinesReferencesByDefault pins the wiring of
// --allow-any-paths through the command itself. Asserting on the loader alone
// would not do it: a mutation making the command always-unconfined leaves the
// loader's own tests green, and that mutant is a working arbitrary-file-read
// server — a cross-sheet reference out of the sheet's directory resolves
// instead of being refused.
func TestServeSheetConfinesReferencesByDefault(t *testing.T) {
	for _, allowAny := range []bool{false, true} {
		captured := captureServeConfig(t)
		args := []string{cmdServe, cmdServeSheet, string(sheetFile(t)), "--port", "0"}
		if allowAny {
			args = append(args, "--"+flagAllowAnyPaths)
		}
		_, err := runCLI(t, args...)
		require.NoError(t, err)
		require.NotNil(t, *captured)
		assert.Equal(t, pathAccess(allowAny), (*captured).isUnconfined,
			"the command passes the operator's choice through, and defaults to confined")
	}
}

// TestServeSheetPassesTheCellCap pins that --max-cells reaches the editing
// session, which is what bounds an edit made in the browser.
func TestServeSheetPassesTheCellCap(t *testing.T) {
	captured := captureServeConfig(t)
	_, err := runCLI(t, "--"+flagMaxCells, "7", cmdServe, cmdServeSheet, string(sheetFile(t)), "--port", "0")
	require.NoError(t, err)
	require.NotNil(t, *captured)
	assert.Equal(t, 7, (*captured).limits.GridDim)
}

// captureServeConfig records what the command decided instead of serving.
func captureServeConfig(t *testing.T) **serveConfig {
	t.Helper()
	var seen *serveConfig
	prev := startServe
	startServe = func(_ context.Context, cfg serveConfig) error {
		seen = &cfg
		return nil
	}
	t.Cleanup(func() { startServe = prev })
	return &seen
}

// TestServeWithoutASurfaceExplainsTheMove pins the one breaking change of this
// command group at the place a user meets it. `tsv serve <sheet>` worked until
// now, so without this it lands in urfave's help-topic path: "No help topic
// for 'sheet.tsvt'" and exit 3 — a code this repo does not use, produced by an
// os.Exit inside the parser that bypasses the exit-code mapping entirely.
func TestServeWithoutASurfaceExplainsTheMove(t *testing.T) {
	prev := stderr
	var notice bytes.Buffer
	stderr = &notice
	t.Cleanup(func() { stderr = prev })

	out, err := runCLI(t, cmdServe, "sheet.tsvt")
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrUsage, "a usage mistake, not a runtime failure")
	assert.Equal(t, exitSyntaxError, exitCode(err))
	assert.Contains(t, notice.String(), "tsv serve sheet sheet.tsvt", "and it names the form that replaces it")
	assert.Contains(t, out, "USAGE:")
}

func TestBareServeShowsItsSurfacesWithoutANotice(t *testing.T) {
	prev := stderr
	var notice bytes.Buffer
	stderr = &notice
	t.Cleanup(func() { stderr = prev })

	out, err := runCLI(t, cmdServe)
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrUsage)
	assert.Empty(t, notice.String(), "nothing was mistyped, so there is nothing to correct")
	assert.Contains(t, out, cmdServeSheet)
	assert.Contains(t, out, cmdServeAPI)
}
