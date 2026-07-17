package cli

import (
	"encoding/csv"
	"html"
	"io"
	"strings"

	"github.com/uplang/go-tsvsheet"

	"github.com/uplang/tsvsheet.go/internal/constants"
)

// Format selects render's output serialization for the computed value grid: TSV
// (the default), CSV, an HTML <table>, or a GitHub-flavored Markdown pipe table.
// One engine, one grid, many renderings.
type Format string

// The render output formats. md is accepted as an alias for markdown; every
// other value is an ErrUnknownFormat.
const (
	formatTSV      Format = "tsv"
	formatCSV      Format = "csv"
	formatHTML     Format = "html"
	formatMarkdown Format = "markdown"
	formatMD       Format = "md"
)

// The --format flag: its name and usage text, bound inline on the render command
// to that command's local Format value.
const (
	flagFormat  = "format"
	usageFormat = "Output format for the computed grid: tsv (default), csv, html, or markdown (md)"
)

// rendering is a fully serialized grid, ready to write to an output stream.
type rendering string

// write emits the rendering to w, wrapping a failure as ErrWriteFile so every
// formatter reports output errors uniformly.
func (r rendering) write(w io.Writer) error {
	if _, err := io.WriteString(w, string(r)); err != nil {
		return tsvsheet.ErrWriteFile.With(err)
	}
	return nil
}

// format writes the computed grid to w in the requested format. An unrecognized
// format is ErrUnknownFormat naming the offending value (matchable with
// errors.Is); the tsv path is byte-identical to tsvsheet.WriteTSV.
func format(w io.Writer, grid tsvsheet.Grid, f Format) error {
	switch f {
	case formatTSV:
		return tsvsheet.WriteTSV(w, grid)
	case formatCSV:
		return writeCSV(w, grid)
	case formatHTML:
		return writeHTML(w, grid)
	case formatMarkdown, formatMD:
		return writeMarkdown(w, grid)
	default:
		return constants.ErrUnknownFormat.With(nil, "format", string(f))
	}
}

// writeCSV writes the grid as RFC 4180 CSV, quoting any cell that contains a
// comma, quote, or newline. A write failure surfaces as ErrWriteFile.
func writeCSV(w io.Writer, grid tsvsheet.Grid) error {
	cw := csv.NewWriter(w)
	if err := cw.WriteAll(grid); err != nil {
		return tsvsheet.ErrWriteFile.With(err)
	}
	return nil
}

// writeHTML writes the grid as a plain, class-tagged HTML <table> — one <tr> per
// row, one <td> per cell, every cell HTML-escaped, no inline styles — so it
// composes with any stylesheet (matching the goldmark extension's output).
func writeHTML(w io.Writer, grid tsvsheet.Grid) error {
	rows := make([]string, len(grid))
	for i, row := range grid {
		rows[i] = htmlRow(row)
	}
	body := `<table class="tsvsheet">` + "\n" + strings.Join(rows, "") + "</table>\n"
	return rendering(body).write(w)
}

// htmlRow renders one grid row as a <tr> of HTML-escaped <td> cells.
func htmlRow(row []string) string {
	cells := make([]string, len(row))
	for i, cell := range row {
		cells[i] = "<td>" + html.EscapeString(cell) + "</td>"
	}
	return "<tr>" + strings.Join(cells, "") + "</tr>\n"
}

// writeMarkdown writes the grid as a GitHub-flavored pipe table: the first row is
// the header, followed by a --- separator row, then the remaining rows as the
// body. Each cell is escaped for pipe-table safety. An empty grid yields no
// output.
func writeMarkdown(w io.Writer, grid tsvsheet.Grid) error {
	if len(grid) == 0 {
		return nil
	}
	rows := make([]string, 0, len(grid)+1)
	rows = append(rows, markdownRow(grid[0]), markdownRow(separatorRow(grid[0])))
	for _, row := range grid[1:] {
		rows = append(rows, markdownRow(row))
	}
	return rendering(strings.Join(rows, "")).write(w)
}

// markdownRow renders one grid row as a pipe-delimited table row. Each cell is
// escaped so no value breaks the table: a | is backslash-escaped so it never
// starts a new column, and a newline — which a cell can hold via CHAR(10) —
// becomes a <br> so it never splits the row into two table lines (GFM renders
// <br> as an in-cell line break).
func markdownRow(row []string) string {
	cells := make([]string, len(row))
	for i, cell := range row {
		escaped := strings.ReplaceAll(cell, "|", `\|`)
		escaped = strings.ReplaceAll(escaped, "\r\n", "<br>")
		escaped = strings.ReplaceAll(escaped, "\n", "<br>")
		cells[i] = strings.ReplaceAll(escaped, "\r", "<br>")
	}
	return "| " + strings.Join(cells, " | ") + " |\n"
}

// separatorRow builds the --- header/body divider matching the header's column
// count.
func separatorRow(header []string) []string {
	cells := make([]string, len(header))
	for i := range cells {
		cells[i] = "---"
	}
	return cells
}
