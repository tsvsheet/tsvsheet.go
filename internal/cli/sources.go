// Package cli is the tsvsheet command tier: it wires the engine (the
// github.com/tsvsheet/go-tsvsheet library) to urfave/cli commands with strict
// unix stdin/stdout discipline. A .tsvt file IS the spreadsheet; every command takes it as a
// positional argument. Command logic lives in stream-injected functions so it
// is fully testable; the cli.Command wrappers only bind flags and streams.
package cli

import (
	"bytes"
	"errors"
	"io"
	"os"

	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

// Streams are a command's injected I/O: input, output, and diagnostics. Real
// runs wire os.Stdin/Stdout/Stderr; tests wire buffers.
type Streams struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

// sourcePath is a positional spreadsheet path. Empty or "-" means stdin.
type sourcePath string

// stdinMarker is the conventional stdin path.
const stdinMarker = "-"

// isStdin reports whether the path selects standard input.
func (p sourcePath) isStdin() bool { return p == "" || p == stdinMarker }

// display names the source for a human: the path as written, and the stdin
// marker for stdin however it was spelled, so an omitted argument and an
// explicit "-" read the same in output.
func (p sourcePath) display() string {
	if p.isStdin() {
		return stdinMarker
	}
	return string(p)
}

// closeFunc releases an opened source; it is a no-op for stdin.
type closeFunc func() error

// open returns a reader for the path, using stdin when the path selects it.
func (p sourcePath) open(stdin io.Reader) (io.Reader, closeFunc, error) {
	if p.isStdin() {
		return stdin, noClose, nil
	}
	file, err := os.Open(string(p))
	if err != nil {
		return nil, nil, constants.ErrOpenFile.With(err)
	}
	return file, file.Close, nil
}

// noClose is the release for a source that must not be closed (stdin).
func noClose() error { return nil }

// readAll reads a source fully into a byte slice, wrapping failures.
func readAll(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, tsvsheet.ErrReadInput.With(err)
	}
	return data, nil
}

// parseDocument reads a spreadsheet source and parses it with its physical
// line layout, so the `#.` view directives survive — a plain sheet drops the
// lines they live on. The load is bounded (spec 018): a file source is
// census-vetted BEFORE buffering (an over-budget 1.3 GB file refuses in
// O(index) memory rather than a gigabyte transient —
// TestReadBounded_VetsFilesBeforeBuffering states this), a byte-buffered source is
// vetted before parsing, and every refusal is built by vetCensus — one
// message, the census, the budget, and the remedies.
func parseDocument(r io.Reader, limits tsvsheet.Limits) (tsvsheet.Document, error) {
	src, err := readBounded(r, limits)
	if err != nil {
		return tsvsheet.Document{}, err
	}
	return tsvsheet.ParseDocument(src)
}

// parseSheet reads a spreadsheet source and parses it, bounded like
// parseDocument.
func parseSheet(r io.Reader, limits tsvsheet.Limits) (tsvsheet.Sheet, error) {
	src, err := readBounded(r, limits)
	if err != nil {
		return tsvsheet.Sheet{}, err
	}
	return tsvsheet.Parse(src)
}

// statReaderAt is the shape a source must have for the pre-buffer census: a
// positional reader with a size. *os.File has it; the interface (rather than
// a concrete *os.File assert) is what lets a test PROVE the vet happens
// before buffering — TestReadBounded_VetsFilesBeforeBuffering's double, whose
// sequential Read explodes, passes the vet through ReadAt without buffering.
type statReaderAt interface {
	io.Reader
	io.ReaderAt
	Stat() (os.FileInfo, error)
}

// censusCheckpoint is the first prefix size a stream is censused at, after
// which the checkpoint doubles. Doubling is what keeps the repeated scans
// amortised to O(n) over the whole read rather than O(n²), while still catching
// an over-budget stream within a factor of two of the bytes that proved it.
const censusCheckpoint = 1 << 20

// readBounded reads a source fully, refusing an over-budget document as early
// as its shape allows: a regular file is census-vetted through ReadAt before
// any buffering, and a stream is censused as it is read. The bytes returned
// are exactly the bytes vetted, so the plain parse that follows cannot exceed
// the budget.
func readBounded(r io.Reader, limits tsvsheet.Limits) ([]byte, error) {
	f, ok := r.(statReaderAt)
	if !ok {
		return readStream(r, limits)
	}
	vetted, err := vetFile(f, limits)
	if err != nil {
		return nil, err
	}
	if !vetted {
		return readStream(r, limits)
	}
	return readVettedFile(r, limits)
}

// readVettedFile buffers a file the census already cleared, then re-vets the
// bytes actually read: the pre-vet measured the file on disk, and this is what
// makes the returned bytes — not the file — the thing that was vetted.
func readVettedFile(r io.Reader, limits tsvsheet.Limits) ([]byte, error) {
	src, err := readAll(r)
	if err != nil {
		return nil, err
	}
	return src, vetBytes(src, limits)
}

// readStream reads a source whose size is unknown until it has been read — a
// pipe, a terminal, an in-memory reader — refusing as soon as the prefix it
// already holds is over budget. A stream has no size to census up front, so the census
// runs on the bytes in hand at growing checkpoints; a prefix's cell count only
// ever grows as more bytes arrive, so a prefix over budget proves the document
// is, and the refusal costs the prefix that proved it rather than the whole
// stream. Without this a `cat huge.tsvt | tsv render` buffered the entire
// document before anything looked at the budget —
// TestReadBounded_RefusesAnOverBudgetStreamOnItsPrefix states the cost bound,
// and TestReadBounded_StreamSpanningManyCheckpoints states that an in-budget
// stream still arrives whole.
func readStream(r io.Reader, limits tsvsheet.Limits) ([]byte, error) {
	var buf bytes.Buffer
	for checkpoint := int64(censusCheckpoint); ; checkpoint *= 2 {
		_, err := io.CopyN(&buf, r, checkpoint-int64(buf.Len()))
		if vetErr := vetBytes(buf.Bytes(), limits); vetErr != nil {
			return nil, vetErr
		}
		if err != nil {
			return buf.Bytes(), streamEnd(err)
		}
	}
}

// streamEnd reads CopyN's terminator: EOF means the stream ran out, which is a
// whole document rather than a failure, and any other error is a read failure —
// TestReadBounded_StreamReadFailureIsAReadError states the failing leg.
func streamEnd(err error) error {
	if errors.Is(err, io.EOF) {
		return nil
	}
	return tsvsheet.ErrReadInput.With(err)
}

// vetBytes census-vets a buffer already in memory.
func vetBytes(src []byte, limits tsvsheet.Limits) error {
	return vetCensus(tsvsheet.ByteSource{ReadAt: bytes.NewReader(src), Size: int64(len(src))}, limits)
}

// vetFile census-vets an open file in place: ReadAt is positional, so the
// scan leaves the file's read offset untouched for the buffering that
// follows an in-budget verdict.
//
// Only a REGULAR file can be vetted this way, and being file-SHAPED is not the
// same thing: os.Stdin is an *os.File and so satisfies statReaderAt whatever it
// is attached to, but on a pipe its ReadAt fails with ESPIPE and its Stat size
// is not the document's. Testing the shape instead of the mode is what broke
// `cat sheet.tsvt | tsv check` (and render, parse, explain) while the seekable
// `tsv check < sheet.tsvt` kept working. A non-regular source is left to the
// buffered vet in readBounded, which is the only order a stream allows.
// It reports whether it vetted, so the caller knows whether the source still
// owes a census as it is read.
func vetFile(f statReaderAt, limits tsvsheet.Limits) (bool, error) {
	info, err := f.Stat()
	if err != nil {
		return false, constants.ErrOpenFile.With(err)
	}
	if !info.Mode().IsRegular() {
		return false, nil
	}
	return true, vetCensus(tsvsheet.ByteSource{ReadAt: f, Size: info.Size()}, limits)
}

// vetCensus refuses a source whose cell count exceeds the resident ceiling
// the engine itself resolves (EffectiveResidentCells — the same single
// ceiling --max-cells sets), with the CLI's remedies attached.
func vetCensus(src tsvsheet.ByteSource, limits tsvsheet.Limits) error {
	census, err := tsvsheet.Census(src)
	if err != nil {
		return err
	}
	budget := limits.EffectiveResidentCells()
	if census.Cells > budget {
		return tsvsheet.ErrDocTooLarge.With(nil,
			"cells", census.Cells, "budget", budget,
			"hint", "raise --max-cells to accept the cost, or view any size with `tsv tui`")
	}
	return nil
}
