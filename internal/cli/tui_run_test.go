package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
	"github.com/tsvsheet/tsvsheet.go/internal/tui"
)

// withRunProgram swaps the tea program runner for a test double.
func withRunProgram(t *testing.T, fn func(tea.Model, io.Reader, io.Writer) error) {
	t.Helper()
	prev := runProgram
	runProgram = fn
	t.Cleanup(func() { runProgram = prev })
}

func TestRunTUI_LoadsAndRuns(t *testing.T) {
	var gotModel tea.Model
	withRunProgram(t, func(m tea.Model, _ io.Reader, _ io.Writer) error {
		gotModel = m
		return nil
	})

	streams := Streams{In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	require.NoError(t, runTUI(streams, tuiConfig{source: sheetFile(t)}))
	assert.NotNil(t, gotModel)
}

func TestRunTUI_RequiresFile(t *testing.T) {
	t.Parallel()

	err := runTUI(Streams{In: strings.NewReader("")}, tuiConfig{source: "-"})
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrInvalidValue)
}

func TestRunTUI_ProgramError(t *testing.T) {
	withRunProgram(t, func(tea.Model, io.Reader, io.Writer) error {
		return errors.New("tea boom")
	})

	err := runTUI(Streams{In: strings.NewReader(""), Out: &bytes.Buffer{}}, tuiConfig{source: sheetFile(t)})
	require.Error(t, err)
}

func TestRunTUI_BadRefreshSpec(t *testing.T) {
	t.Parallel()

	// A malformed --refresh-interval fails before the program runs: runTUI
	// surfaces the buildRefresh error rather than starting the editor.
	err := runTUI(
		Streams{In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &bytes.Buffer{}},
		tuiConfig{source: sheetFile(t), refresh: "garbage!!!"},
	)
	require.Error(t, err)
}

func TestTUICommand_Integration(t *testing.T) {
	withRunProgram(t, func(tea.Model, io.Reader, io.Writer) error { return nil })

	cmd := tuiCommand()
	err := cmd.Run(context.Background(), []string{cmdTUI, string(sheetFile(t))})
	require.NoError(t, err)
}

func TestDefaultRunProgram_QuitsOnInput(t *testing.T) {
	// Drive the real (unswapped) runProgram headlessly: feed "q" so the model
	// quits and Run returns, exercising the default tea.Program path without a
	// TTY.
	streams := Streams{In: strings.NewReader("q"), Out: io.Discard, Err: io.Discard}
	require.NoError(t, runTUI(streams, tuiConfig{source: sheetFile(t)}))
}

// tinyLimits is a cell budget small enough that any real sheet file pages.
func tinyLimits() tsvsheet.Limits {
	return tsvsheet.Limits{ResultCells: 1, GridDim: 100, ResultBytes: 100}
}

// overBudgetFile writes a sheet whose census exceeds tinyLimits' one-cell
// resident fallback.
func overBudgetFile(t *testing.T) sourcePath {
	t.Helper()
	path := filepath.Join(t.TempDir(), "big.tsvt")
	require.NoError(t, os.WriteFile(path, []byte("a\tb\n=A1&B1\tc\n"), 0o600))
	return sourcePath(path)
}

// TestRunTUI_PagesAnOverBudgetDocument pins the 016 routing: a document over
// the resident cell budget runs the view-only pager, never the editor — and
// the same file under default limits still gets the editor, so the budget is
// the only thing that decides.
func TestRunTUI_PagesAnOverBudgetDocument(t *testing.T) {
	var gotModel tea.Model
	withRunProgram(t, func(m tea.Model, _ io.Reader, _ io.Writer) error {
		gotModel = m
		return nil
	})
	streams := Streams{In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	source := overBudgetFile(t)

	require.NoError(t, runTUI(streams, tuiConfig{source: source, limits: tinyLimits()}))
	_, isPager := gotModel.(tui.Pager)
	assert.True(t, isPager, "over budget must page, got %T", gotModel)

	require.NoError(t, runTUI(streams, tuiConfig{source: source}))
	_, isPager = gotModel.(tui.Pager)
	assert.False(t, isPager, "the same file in budget must edit, got %T", gotModel)
}

// TestRunTUI_PagerEndToEndQuits drives the over-budget path through the real
// tea program headlessly: "q" ends it cleanly (frame content is pinned at the
// model level in the tui package; a headless run may quit before rendering).
func TestRunTUI_PagerEndToEndQuits(t *testing.T) {
	streams := Streams{In: strings.NewReader("q"), Out: io.Discard, Err: io.Discard}
	require.NoError(t, runTUI(streams, tuiConfig{source: overBudgetFile(t), limits: tinyLimits()}))
}

// framingRunner substitutes runProgram with a runner that sizes the model and
// captures its first rendered frame — inside the program run, while the
// pager's source file is still open.
func framingRunner(t *testing.T, frame *string) func(tea.Model, io.Reader, io.Writer) error {
	t.Helper()
	return func(m tea.Model, _ io.Reader, _ io.Writer) error {
		sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
		*frame = sized.(tui.Pager).View()
		return nil
	}
}

// TestRunTUI_PagerCarriesTheCallersLimits kills the adversary's surviving
// mutation: the pager's ComputeOptions must carry cfg.limits, so a formula
// over the touched-cells budget answers #LIMIT! in the frame — zeroed options
// would fall back to DefaultLimits and compute it.
func TestRunTUI_PagerCarriesTheCallersLimits(t *testing.T) {
	var frame string
	withRunProgram(t, framingRunner(t, &frame))
	path := filepath.Join(t.TempDir(), "limited.tsvt")
	require.NoError(t, os.WriteFile(path, []byte("=A2+A3\n1\n2\n"), 0o600))

	streams := Streams{In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	require.NoError(t, runTUI(streams, tuiConfig{source: sourcePath(path), limits: tinyLimits()}))
	assert.Contains(t, frame, "#LIMIT!",
		"the one-cell touched budget must refuse the two-cell walk")
}

// recordingFetcher is a Fetcher fake that reports whether it was called.
type recordingFetcher struct{ called bool }

func (r *recordingFetcher) Fetch(_ tsvsheet.ImportURL, accept tsvsheet.MediaType) (tsvsheet.FetchResult, error) {
	r.called = true
	return tsvsheet.FetchResult{ContentType: accept, Body: []byte("42")}, nil
}

// TestRunTUI_PagerCarriesTheCallersFetcher kills the other half of the same
// surviving mutation: an IMPORT cell in the visible window must fetch through
// cfg.fetcher — zeroed options carry a nil Fetcher and answer #IMPORT!.
func TestRunTUI_PagerCarriesTheCallersFetcher(t *testing.T) {
	var frame string
	withRunProgram(t, framingRunner(t, &frame))
	path := filepath.Join(t.TempDir(), "imported.tsvt")
	require.NoError(t, os.WriteFile(path, []byte("=importcell(\"https://x/v\")\nx\n"), 0o600))

	fetcher := &recordingFetcher{}
	limits := tsvsheet.DefaultLimits()
	limits.ResidentCells = 1
	streams := Streams{In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	require.NoError(t, runTUI(streams, tuiConfig{source: sourcePath(path), limits: limits, fetcher: fetcher}))
	assert.True(t, fetcher.called, "the import must fetch through the injected fetcher")
	assert.Contains(t, frame, "42", "the fetched value renders in the window")
}

// TestOpenWindowed_StdinPassesThrough pins that stdin skips the probe so
// loadEditable's own file-required message stays the one the user sees.
func TestOpenWindowed_StdinPassesThrough(t *testing.T) {
	t.Parallel()

	windowed, closeSource := openWindowed("-", tinyLimits())
	assert.Nil(t, windowed)
	assert.Nil(t, closeSource)
}

// TestOpenWindowed_InBudgetIsNilAndClosesTheFile pins the common case: an
// in-budget file reports nothing windowed, and the descriptor it probed with
// is closed — proven through the captured file, not asserted by comment.
func TestOpenWindowed_InBudgetIsNilAndClosesTheFile(t *testing.T) {
	prev := statSize
	t.Cleanup(func() { statSize = prev })
	var captured *os.File
	statSize = func(f *os.File) (int64, error) {
		captured = f
		return prev(f)
	}

	windowed, closeSource := openWindowed(sheetFile(t), tsvsheet.DefaultLimits())
	assert.Nil(t, windowed)
	assert.Nil(t, closeSource)
	require.NotNil(t, captured)
	_, statErr := captured.Stat()
	assert.ErrorIs(t, statErr, os.ErrClosed, "the in-budget probe must close its descriptor")
}

// TestOpenWindowed_MissingFileFallsThrough pins the fall-through contract: a
// file the probe cannot open reports nothing windowed, and runTUI surfaces
// loadEditable's own pre-016 sentinel.
func TestOpenWindowed_MissingFileFallsThrough(t *testing.T) {
	t.Parallel()

	absent := sourcePath(filepath.Join(t.TempDir(), "absent.tsvt"))
	windowed, closeSource := openWindowed(absent, tinyLimits())
	assert.Nil(t, windowed)
	assert.Nil(t, closeSource)

	err := runTUI(Streams{In: strings.NewReader("")}, tuiConfig{source: absent})
	assert.ErrorIs(t, err, constants.ErrOpenFile)
}

// TestOpenWindowed_DirectoryKeepsTheOldErrorSurface pins the adversary's
// finding: a directory source must fail exactly as it always has — through
// loadEditable's os.ReadFile with ErrOpenFile — never with a new engine
// sentinel from the probe.
func TestOpenWindowed_DirectoryKeepsTheOldErrorSurface(t *testing.T) {
	t.Parallel()

	dir := sourcePath(t.TempDir())
	windowed, closeSource := openWindowed(dir, tinyLimits())
	assert.Nil(t, windowed)
	assert.Nil(t, closeSource)

	err := runTUI(Streams{In: strings.NewReader("")}, tuiConfig{source: dir})
	assert.ErrorIs(t, err, constants.ErrOpenFile)
}

// TestOpenWindowed_StatFailureClosesTheFile forces the stat branch through the
// seam and proves the fall-through leaks no descriptor: after the failure the
// captured file is already closed and nothing windowed is reported.
func TestOpenWindowed_StatFailureClosesTheFile(t *testing.T) {
	prev := statSize
	t.Cleanup(func() { statSize = prev })
	var captured *os.File
	statSize = func(f *os.File) (int64, error) {
		captured = f
		return 0, assert.AnError
	}

	windowed, closeSource := openWindowed(overBudgetFile(t), tinyLimits())
	assert.Nil(t, windowed)
	assert.Nil(t, closeSource)
	require.NotNil(t, captured)
	_, statErr := captured.Stat()
	assert.ErrorIs(t, statErr, os.ErrClosed, "the failed probe must close its descriptor")
}

// TestOpenWindowed_ScanRefusalFallsThroughClosed pins the engine-error branch
// with a real refusal — a line past the engine's scan ceiling — and proves the
// fall-through closes the descriptor while runTUI surfaces the editable
// path's own refusal for the same file.
func TestOpenWindowed_ScanRefusalFallsThroughClosed(t *testing.T) {
	prev := statSize
	t.Cleanup(func() { statSize = prev })
	var captured *os.File
	statSize = func(f *os.File) (int64, error) {
		captured = f
		return prev(f)
	}

	path := filepath.Join(t.TempDir(), "longline.tsvt")
	require.NoError(t, os.WriteFile(path, append(bytes.Repeat([]byte("a"), (1<<20)+2), '\n'), 0o600))
	windowed, closeSource := openWindowed(sourcePath(path), tsvsheet.DefaultLimits())
	assert.Nil(t, windowed)
	assert.Nil(t, closeSource)
	require.NotNil(t, captured)
	_, statErr := captured.Stat()
	assert.ErrorIs(t, statErr, os.ErrClosed, "the failed probe must close its descriptor")

	err := runTUI(Streams{In: strings.NewReader("")}, tuiConfig{source: sourcePath(path)})
	assert.ErrorIs(t, err, tsvsheet.ErrReadInput, "the refusal is loadEditable's own")
}
