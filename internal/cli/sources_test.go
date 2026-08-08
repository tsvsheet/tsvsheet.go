package cli

import (
	"bytes"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

// TestNoClose_LeavesStdinOpen pins why the stdin release is a no-op rather than
// a Close: a command that closed os.Stdin would break the next stage of a
// pipeline, and every caller defers the release unconditionally — the no-op is
// what makes that unconditional defer safe.
func TestNoClose_LeavesStdinOpen(t *testing.T) {
	t.Parallel()

	assert.NoError(t, noClose(), "the stdin release never reports failure")

	in := strings.NewReader("a\tb\n")
	reader, release, err := sourcePath("-").open(in)
	require.NoError(t, err)
	require.NoError(t, release(), "releasing stdin is a no-op")

	// Still readable after the release; a real Close would have broken this.
	rest, err := readAll(reader)
	require.NoError(t, err)
	assert.Equal(t, "a\tb\n", string(rest))
}

// TestSourcePath_EmptyAndDashBothMeanStdin pins the two spellings a command
// accepts for "read standard input", so an omitted positional and an explicit
// "-" cannot diverge.
func TestSourcePath_EmptyAndDashBothMeanStdin(t *testing.T) {
	t.Parallel()

	assert.True(t, sourcePath("").isStdin())
	assert.True(t, sourcePath("-").isStdin())
	assert.False(t, sourcePath("sheet.tsvt").isStdin())
}

// TestParseHelpers_RefuseOverBudgetWithTheHint pins the 018 CLI contract at
// the shared seam: both bounded helpers refuse an over-resident source as
// ErrDocTooLarge carrying the CLI's own remedies (the flag and the TUI), and
// pass every other outcome through untouched.
func TestParseHelpers_RefuseOverBudgetWithTheHint(t *testing.T) {
	t.Parallel()

	over := strings.NewReader("a\tb\nc\td\n")
	_, err := parseSheet(over, tsvsheet.Limits{ResidentCells: 1})
	require.ErrorIs(t, err, tsvsheet.ErrDocTooLarge)
	assert.Contains(t, err.Error(), "--max-cells", "the refusal names the flag")
	assert.Contains(t, err.Error(), "tsv tui", "the refusal names the any-size viewer")

	_, err = parseDocument(strings.NewReader("a\tb\nc\td\n"), tsvsheet.Limits{ResidentCells: 1})
	require.ErrorIs(t, err, tsvsheet.ErrDocTooLarge)
	assert.Contains(t, err.Error(), "--max-cells")

	sheet, err := parseSheet(strings.NewReader("a\tb\n"), tsvsheet.DefaultLimits())
	require.NoError(t, err)
	assert.Equal(t, tsvsheet.Grid{{"a", "b"}}, sheet.Source(), "in budget parses as before")

	_, err = parseSheet(strings.NewReader("=)(\n"), tsvsheet.DefaultLimits())
	require.ErrorIs(t, err, tsvsheet.ErrSyntax, "a non-budget failure passes through undecorated")
	assert.NotContains(t, err.Error(), "--max-cells")
}

// explodingFile is a file-shaped source whose census works through ReadAt but
// whose sequential Read fails loudly — the discriminator proving the vet
// happens BEFORE buffering (the O(index) refusal property): a code path that
// buffered first would trip the explosion.
type explodingFile struct {
	data *bytes.Reader
}

func (e explodingFile) Read([]byte) (int, error) { return 0, assert.AnError }
func (e explodingFile) ReadAt(p []byte, off int64) (int, error) {
	return e.data.ReadAt(p, off)
}

func (e explodingFile) Stat() (os.FileInfo, error) {
	return fakeInfo{size: e.data.Size()}, nil
}

// fakeInfo carries only the size and mode the vet consults. A zero mode is a
// regular file, which is what the file-shaped doubles want.
type fakeInfo struct {
	size int64
	mode os.FileMode
}

func (f fakeInfo) Name() string       { return "exploding.tsvt" }
func (f fakeInfo) Size() int64        { return f.size }
func (f fakeInfo) Mode() os.FileMode  { return f.mode }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return false }
func (f fakeInfo) Sys() any           { return nil }

// TestReadBounded_VetsFilesBeforeBuffering kills the adversary's surviving
// mutation: an over-budget file-shaped source must refuse through the census
// alone — its sequential Read (the buffering) is never called, which is the
// 229MB-instead-of-3GB property stated as a test.
func TestReadBounded_VetsFilesBeforeBuffering(t *testing.T) {
	t.Parallel()

	over := explodingFile{data: bytes.NewReader([]byte("a\tb\nc\td\n"))}
	_, err := readBounded(over, tsvsheet.Limits{ResidentCells: 1})
	require.ErrorIs(t, err, tsvsheet.ErrDocTooLarge,
		"the refusal must come from the census, not from buffering")
	assert.NotErrorIs(t, err, tsvsheet.ErrReadInput,
		"the exploding sequential Read was never reached")
}

// pipeFile is what os.Stdin actually is on `cat sheet.tsvt | tsv check`: an
// *os.File, and so file-SHAPED, attached to a pipe. Its ReadAt fails the way a
// pipe's does (ESPIPE), so any code that positionally vets it is caught here
// rather than by a user. Its sequential Read is the only one that works —
// which is the whole point of a stream.
type pipeFile struct{ data *bytes.Reader }

func (p pipeFile) Read(b []byte) (int, error) { return p.data.Read(b) }

func (p pipeFile) ReadAt([]byte, int64) (int, error) {
	return 0, syscall.ESPIPE
}

func (p pipeFile) Stat() (os.FileInfo, error) {
	return fakeInfo{size: 0, mode: os.ModeNamedPipe}, nil
}

// TestReadBounded_StreamsAPipeInsteadOfSeekingIt pins the regression that broke
// every piped command: a file-shaped source that is not a REGULAR file must be
// read sequentially, never positionally. Vetting it through ReadAt is what
// turned `cat sheet.tsvt | tsv check` into "illegal seek" while the seekable
// `tsv check < sheet.tsvt` kept working.
func TestReadBounded_StreamsAPipeInsteadOfSeekingIt(t *testing.T) {
	t.Parallel()

	src, err := readBounded(pipeFile{data: bytes.NewReader([]byte("a\tb\n"))}, tsvsheet.DefaultLimits())
	require.NoError(t, err, "a pipe must be streamed, not seeked")
	assert.Equal(t, "a\tb\n", string(src))
}

// TestReadBounded_VetsAPipeAfterBuffering states the budget still binds on a
// stream: the census cannot run before buffering when the source cannot seek,
// so it runs after — but it does run, and the refusal is the ordinary
// over-budget one rather than a seek failure.
func TestReadBounded_VetsAPipeAfterBuffering(t *testing.T) {
	t.Parallel()

	_, err := readBounded(pipeFile{data: bytes.NewReader([]byte("a\tb\nc\td\n"))}, tsvsheet.Limits{ResidentCells: 1})
	require.ErrorIs(t, err, tsvsheet.ErrDocTooLarge)
	assert.NotErrorIs(t, err, syscall.ESPIPE, "the budget refused it, not the seek")
}

// statFailingFile is file-shaped but cannot be statted.
type statFailingFile struct{ explodingFile }

func (statFailingFile) Stat() (os.FileInfo, error) { return nil, assert.AnError }

// TestReadBounded_StatFailureIsAnOpenError pins vetFile's failure leg: a
// file-shaped source that cannot be statted refuses as ErrOpenFile with the
// cause preserved.
func TestReadBounded_StatFailureIsAnOpenError(t *testing.T) {
	t.Parallel()

	src := statFailingFile{explodingFile{data: bytes.NewReader([]byte("a\n"))}}
	_, err := readBounded(src, tsvsheet.DefaultLimits())
	require.ErrorIs(t, err, constants.ErrOpenFile)
	require.ErrorIs(t, err, assert.AnError)
}
