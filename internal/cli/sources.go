// Package cli is the tsvsheet command tier: it wires the engine
// (internal/tsvt, internal/sheet) to urfave/cli commands with strict unix
// stdin/stdout discipline. Command logic lives in stream-injected functions so
// it is fully testable; the cli.Command wrappers only bind flags and streams.
package cli

import (
	"io"
	"os"

	"github.com/uplang/tsvsheet.go/internal/constants"
)

// Streams are a command's injected I/O: input, output, and diagnostics. Real
// runs wire os.Stdin/Stdout/Stderr; tests wire buffers.
type Streams struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

// sourcePath is a --template or --data flag value. Empty or "-" means stdin.
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
		return nil, nil, constants.ErrOpenFile.With(err, string(p))
	}
	return file, file.Close, nil
}

// noClose is the release for a source that must not be closed (stdin).
func noClose() error { return nil }

// templateAndData opens both the template and the data sources, rejecting the
// impossible case of reading both from a single stdin stream.
func templateAndData(tmpl, data sourcePath, stdin io.Reader) (io.Reader, io.Reader, closeFunc, error) {
	if tmpl.isStdin() && data.isStdin() {
		return nil, nil, nil, constants.ErrInvalidValue.With(nil, "message", "cannot read both template and data from stdin; give a path for one")
	}
	tmplReader, closeTmpl, err := tmpl.open(stdin)
	if err != nil {
		return nil, nil, nil, err
	}
	dataReader, closeData, err := data.open(stdin)
	if err != nil {
		_ = closeTmpl()
		return nil, nil, nil, err
	}
	return tmplReader, dataReader, chain(closeTmpl, closeData), nil
}

// chain composes two release functions into one, releasing both. Callers defer
// and discard the result (these are read-only file closes after a full read),
// so a close error is benign and not surfaced.
func chain(first, second closeFunc) closeFunc {
	return func() error {
		_ = first()
		_ = second()
		return nil
	}
}

// readAll reads a source fully into a byte slice, wrapping failures.
func readAll(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, constants.ErrReadInput.With(err)
	}
	return data, nil
}
