// Package cli is the tsvsheet command tier: it wires the engine (the
// github.com/tsvsheet/go-tsvsheet library) to urfave/cli commands with strict
// unix stdin/stdout discipline. A .tsvt file IS the spreadsheet; every command takes it as a
// positional argument. Command logic lives in stream-injected functions so it
// is fully testable; the cli.Command wrappers only bind flags and streams.
package cli

import (
	"bytes"
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

// readBounded reads a source fully, refusing an over-budget document as early
// as its shape allows: a file-shaped source is census-vetted through ReadAt
// before any buffering; anything else buffers first and is vetted before the
// caller parses. The bytes returned are exactly the bytes vetted, so the
// plain parse that follows cannot exceed the budget.
func readBounded(r io.Reader, limits tsvsheet.Limits) ([]byte, error) {
	if f, ok := r.(statReaderAt); ok {
		if err := vetFile(f, limits); err != nil {
			return nil, err
		}
	}
	src, err := readAll(r)
	if err != nil {
		return nil, err
	}
	return src, vetCensus(tsvsheet.ByteSource{ReadAt: bytes.NewReader(src), Size: int64(len(src))}, limits)
}

// vetFile census-vets an open file in place: ReadAt is positional, so the
// scan leaves the file's read offset untouched for the buffering that
// follows an in-budget verdict.
func vetFile(f statReaderAt, limits tsvsheet.Limits) error {
	info, err := f.Stat()
	if err != nil {
		return constants.ErrOpenFile.With(err)
	}
	return vetCensus(tsvsheet.ByteSource{ReadAt: f, Size: info.Size()}, limits)
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
