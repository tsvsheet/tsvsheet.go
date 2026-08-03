package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
	"github.com/tsvsheet/tsvsheet.go/internal/tui"
)

// pagedOpen opens path windowed under tinyLimits and fails the test if the
// document did not page.
func pagedOpen(t *testing.T, path string) (*tsvsheet.WindowedSheet, func() error) {
	t.Helper()
	windowed, drift, closeSource := openWindowed(sourcePath(path), tinyLimits())
	require.NotNil(t, windowed)
	require.NotNil(t, drift)
	t.Cleanup(func() { _ = closeSource() })
	return windowed, drift
}

// TestDriftGuard_RefusesTruncateAndRewrite pins 017's acceptance criteria: a
// file truncated or rewritten in place while paged trips the guard with
// ErrSourceChanged before any window is served.
func TestDriftGuard_RefusesTruncateAndRewrite(t *testing.T) {
	t.Parallel()

	truncated := filepath.Join(t.TempDir(), "trunc.tsvt")
	require.NoError(t, os.WriteFile(truncated, []byte("a\tb\nc\td\ne\tf\n"), 0o600))
	_, drift := pagedOpen(t, truncated)
	require.NoError(t, drift(), "an untouched source passes the guard")
	info, err := os.Stat(truncated)
	require.NoError(t, err)
	require.NoError(t, os.Truncate(truncated, 0))
	// Restore the original mtime so ONLY the size leg can catch this — the
	// discriminator for the guard's size comparison (an mtime-forging writer,
	// or a coarse-mtime filesystem, leaves size as the sole witness).
	require.NoError(t, os.Chtimes(truncated, info.ModTime(), info.ModTime()))
	err = drift()
	assert.ErrorIs(t, err, constants.ErrSourceChanged, "truncation must refuse on size alone")
	assert.Contains(t, err.Error(), "size", "the detail names the drifted size")

	rewritten := filepath.Join(t.TempDir(), "rewrite.tsvt")
	require.NoError(t, os.WriteFile(rewritten, []byte("a\tb\nc\td\ne\tf\n"), 0o600))
	_, drift = pagedOpen(t, rewritten)
	require.NoError(t, drift())
	// Same length, different bytes, in place: only mtime betrays it. The
	// rewrite's own timestamp can land inside the filesystem's mtime granule
	// (observed on the CI bind mount), so the test sets a distinct mtime
	// explicitly — any real editor save moves it at least this much — keeping
	// this the discriminator for the guard's mtime leg.
	require.NoError(t, os.WriteFile(rewritten, []byte("x\ty\nz\tw\nq\tr\n"), 0o600))
	require.NoError(t, os.Chtimes(rewritten, time.Now().Add(time.Second), time.Now().Add(time.Second)))
	err = drift()
	assert.ErrorIs(t, err, constants.ErrSourceChanged, "an in-place rewrite must refuse")
	assert.Contains(
		t,
		err.Error(),
		"modified",
		"the detail names what actually drifted — the timestamp, not an unchanged size",
	)
	assert.NotContains(t, err.Error(), "size", "an unchanged size is not offered as the evidence")
}

// TestDriftGuard_RenameReplaceKeepsTheSnapshot pins the deliberate other
// half: an atomic rename-replace swaps the path to a NEW inode, the held
// descriptor keeps the old one, and paging continues on the opened snapshot
// without error — the same consistency the pre-016 editor's read-once load
// gave.
func TestDriftGuard_RenameReplaceKeepsTheSnapshot(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "sheet.tsvt")
	require.NoError(t, os.WriteFile(path, []byte("old1\nold2\nold3\n"), 0o600))
	windowed, drift := pagedOpen(t, path)

	replacement := filepath.Join(dir, "replacement.tsvt")
	require.NoError(t, os.WriteFile(replacement, []byte("new\n"), 0o600))
	require.NoError(t, os.Rename(replacement, path))

	require.NoError(t, drift(), "the held inode has not drifted")
	rows, err := windowed.Rows(0, 3)
	require.NoError(t, err)
	assert.Equal(t, tsvsheet.Grid{{"old1"}, {"old2"}, {"old3"}}, rows,
		"the pager keeps serving the snapshot it opened")
}

// TestRunTUI_WiresTheDriftGuardIntoThePager kills the wiring mutation: runTUI
// must hand openWindowed's guard to the pager, so a truncate between frames
// renders the reopen refusal — a nil guard would render a chimera window.
func TestRunTUI_WiresTheDriftGuardIntoThePager(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mutating.tsvt")
	require.NoError(t, os.WriteFile(path, []byte("a\tb\nc\td\n"), 0o600))

	var frame string
	withRunProgram(t, func(m tea.Model, _ io.Reader, _ io.Writer) error {
		require.NoError(t, os.Truncate(path, 0))
		sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
		frame = sized.(tui.Pager).View()
		return nil
	})
	streams := Streams{In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	require.NoError(t, runTUI(streams, tuiConfig{source: sourcePath(path), limits: tinyLimits()}))
	assert.Contains(t, frame, constants.ErrSourceChanged.Error(),
		"the frame names the change and asks for a reopen")
	assert.Contains(t, frame, "q quits")
}

// TestDriftGuard_StatFailureRefuses pins the guard's third leg: when the held
// descriptor can no longer be statted at all, the guard refuses with the same
// reopen sentinel rather than guessing the source is intact.
func TestDriftGuard_StatFailureRefuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gone.tsvt")
	require.NoError(t, os.WriteFile(path, []byte("a\tb\nc\td\n"), 0o600))
	_, drift := pagedOpen(t, path)
	require.NoError(t, drift())

	prev := statSource
	t.Cleanup(func() { statSource = prev })
	statSource = func(*os.File) (byteSize, time.Time, error) {
		return 0, time.Time{}, assert.AnError
	}
	err := drift()
	assert.ErrorIs(t, err, constants.ErrSourceChanged)
	assert.ErrorIs(t, err, assert.AnError, "the stat cause stays matchable")
}
