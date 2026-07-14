// Package sheet is the tsvsheet processor: it loads a .tsv value grid, applies a
// parsed .tsvt template (internal/tsvt) per SPECIFICATION §9 with the semantics
// fixed in specs/decisions/0003-open-semantics.md, and emits the computed grid.
package sheet

import (
	"bufio"
	"io"
	"strings"

	"github.com/uplang/tsvsheet.go/internal/constants"
)

// tab is the single field separator; newline terminates a row.
const (
	tab     = "\t"
	newline = "\n"
)

// Grid is a rectangular value grid indexed [row][col], 0-based. Cells are raw
// strings; the .tsv side carries no formulas (§2).
type Grid [][]string

// ReadTSV reads a tab-separated value grid. Rows are newline-separated; a
// trailing newline does not add an empty row. A read failure surfaces as
// constants.ErrReadInput.
func ReadTSV(r io.Reader) (Grid, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), maxLineBytes)

	grid := Grid{}
	for scanner.Scan() {
		grid = append(grid, strings.Split(scanner.Text(), tab))
	}
	if err := scanner.Err(); err != nil {
		return nil, constants.ErrReadInput.With(err)
	}
	return grid, nil
}

// maxLineBytes bounds a single scanned row (1 MiB) so a pathological input
// cannot exhaust memory silently.
const maxLineBytes = 1 << 20

// WriteTSV writes the grid as tab-separated rows, each terminated by a newline.
// A write failure surfaces as constants.ErrWriteFile. Callers wanting buffering
// pass a bufio.Writer; WriteTSV writes each row directly so a write error is
// reported at its source.
func WriteTSV(w io.Writer, g Grid) error {
	for _, row := range g {
		if _, err := io.WriteString(w, strings.Join(row, tab)+newline); err != nil {
			return constants.ErrWriteFile.With(err)
		}
	}
	return nil
}

// rows is the grid's row count.
func (g Grid) rows() int { return len(g) }

// cols is the width of the widest row; the grid is treated as ragged-safe by
// reading missing trailing cells as empty (see at).
func (g Grid) cols() int {
	widest := 0
	for _, row := range g {
		if len(row) > widest {
			widest = len(row)
		}
	}
	return widest
}

// at reads the cell at (row, col), returning empty for any out-of-grid or
// ragged-missing position; the boolean reports whether the position is within
// the grid's row/col bounds (a present-but-empty cell is in-bounds).
func (g Grid) at(row, col int) (string, bool) {
	if row < 0 || row >= g.rows() || col < 0 || col >= g.cols() {
		return "", false
	}
	if col >= len(g[row]) {
		return "", true
	}
	return g[row][col], true
}

// clone returns a deep copy so computation never mutates the input grid.
func (g Grid) clone() Grid {
	out := make(Grid, len(g))
	for i, row := range g {
		out[i] = append([]string(nil), row...)
	}
	return out
}
