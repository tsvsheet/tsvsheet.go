package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

func TestRunCheck_Clean(t *testing.T) {
	t.Parallel()

	streams, _, errBuf := streamsWith(sampleSheet)
	require.NoError(t, runCheck(streams, checkSources{"-"}, tsvsheet.DefaultLimits()))
	assert.Empty(t, errBuf.String())
}

func TestRunCheck_Diagnostics(t *testing.T) {
	t.Parallel()

	streams, _, errBuf := streamsWith("=bogus(A1)\n")
	err := runCheck(streams, checkSources{"-"}, tsvsheet.DefaultLimits())
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrDiagnostics)
	assert.Contains(t, errBuf.String(), "A1: unknown function: bogus")
}

func TestRunCheck_SyntaxError(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("1\t=sum(\n")
	err := runCheck(streams, checkSources{"-"}, tsvsheet.DefaultLimits())
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrSyntax)
}

func TestRunCheck_FileMissing(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("")
	err := runCheck(streams, checkSources{"/no/such.tsvt"}, tsvsheet.DefaultLimits())
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrOpenFile)
}

func TestCLI_CheckClean(t *testing.T) {
	withStdin(t, sampleSheet)
	_, err := runCLI(t, cmdCheck)
	require.NoError(t, err)
}

// TestRunCheckLaterSheetIsChecked pins the contract that made this a gate worth
// having: a sheet after the first is checked, not silently dropped. A run that
// reported success here would pass CI over files it never opened.
func TestRunCheckLaterSheetIsChecked(t *testing.T) {
	t.Parallel()

	clean := writeTemp(t, "clean.tsvt", sampleSheet)
	dirty := writeTemp(t, "dirty.tsvt", "=bogus(A1)\n")

	streams, _, errBuf := streamsWith("")
	err := runCheck(streams, checkSources{sourcePath(clean), sourcePath(dirty)}, tsvsheet.DefaultLimits())
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrDiagnostics)
	assert.Contains(t, errBuf.String(), dirty+":A1: unknown function: bogus")
}

// TestRunCheckContinuesPastAFailingSheet asserts a failing sheet does not end
// the run: a gate that stopped at the first would hide every sheet after it.
func TestRunCheckContinuesPastAFailingSheet(t *testing.T) {
	t.Parallel()

	first := writeTemp(t, "first.tsvt", "=bogus(A1)\n")
	second := writeTemp(t, "second.tsvt", "=alsobogus(A1)\n")

	streams, _, errBuf := streamsWith("")
	err := runCheck(streams, checkSources{sourcePath(first), sourcePath(second)}, tsvsheet.DefaultLimits())
	require.ErrorIs(t, err, constants.ErrDiagnostics)
	assert.Contains(t, errBuf.String(), first+":A1: unknown function: bogus")
	assert.Contains(t, errBuf.String(), second+":A1: unknown function: alsobogus")
}

// TestRunCheckSyntaxErrorOutranksDiagnostics asserts the exit code reflects the
// severest sheet wherever it sits in the argument list, so a syntax error is
// never masked by a merely-diagnosed sheet beside it.
func TestRunCheckSyntaxErrorOutranksDiagnostics(t *testing.T) {
	t.Parallel()

	diagnosed := writeTemp(t, "diagnosed.tsvt", "=bogus(A1)\n")
	broken := writeTemp(t, "broken.tsvt", "1\t=sum(\n")

	for name, sources := range map[string]checkSources{
		"syntax first": {sourcePath(broken), sourcePath(diagnosed)},
		"syntax last":  {sourcePath(diagnosed), sourcePath(broken)},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			streams, _, _ := streamsWith("")
			assert.ErrorIs(t, runCheck(streams, sources, tsvsheet.DefaultLimits()), tsvsheet.ErrSyntax)
		})
	}
}

// TestRunCheckUnreadableSheetOutranksDiagnostics asserts an unopenable sheet
// decides the outcome over a merely-diagnosed one: it was never checked at all,
// which is the more serious thing to have happened.
func TestRunCheckUnreadableSheetOutranksDiagnostics(t *testing.T) {
	t.Parallel()

	diagnosed := writeTemp(t, "diagnosed.tsvt", "=bogus(A1)\n")

	streams, _, _ := streamsWith("")
	err := runCheck(streams, checkSources{sourcePath(diagnosed), "/no/such.tsvt"}, tsvsheet.DefaultLimits())
	assert.ErrorIs(t, err, constants.ErrOpenFile)
}

// TestCheckSourcesLabel asserts diagnostics are prefixed only when more than
// one sheet is in play, so a single-sheet run keeps the terse addressing every
// existing caller and doc example depends on.
func TestCheckSourcesLabel(t *testing.T) {
	t.Parallel()

	assert.Empty(t, checkSources{"only.tsvt"}.label("only.tsvt"))
	assert.Equal(t, diagnosticLabel("a.tsvt:"), checkSources{"a.tsvt", "b.tsvt"}.label("a.tsvt"))
	assert.Equal(t, diagnosticLabel("-:"), checkSources{"-", "b.tsvt"}.label(""))
}

// TestPositionalSheets asserts an absent argument still yields exactly one
// source — stdin — and that every argument given becomes a source.
func TestPositionalSheets(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []sourcePath{""}, positional(nil).sheets())
	assert.Equal(t, []sourcePath{"a.tsvt", "b.tsvt"}, positional{"a.tsvt", "b.tsvt"}.sheets())
}

// TestSourcePathDisplay asserts both spellings of stdin present identically, so
// an omitted argument and an explicit "-" label the same way.
func TestSourcePathDisplay(t *testing.T) {
	t.Parallel()

	assert.Equal(t, stdinMarker, sourcePath("").display())
	assert.Equal(t, stdinMarker, sourcePath(stdinMarker).display())
	assert.Equal(t, "sheet.tsvt", sourcePath("sheet.tsvt").display())
}
