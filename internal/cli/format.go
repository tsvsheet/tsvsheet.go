package cli

import (
	"encoding/csv"
	"html"
	"io"
	"strings"

	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
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
	flagHidden  = "hidden"
	flagCell    = "cell"
	usageHidden = "What to do with rows and columns the sheet hides: keep (default) or drop"
	usageCell   = "Print only this cell's computed value, verbatim (A1 notation), instead of the grid"
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
func format(w io.Writer, seen projection, f Format) error {
	switch f {
	case formatTSV:
		return tsvsheet.WriteTSV(w, seen.grid)
	case formatCSV:
		return writeCSV(w, seen.grid)
	case formatHTML:
		return writeHTML(w, seen)
	case formatMarkdown, formatMD:
		return writeMarkdown(w, seen.grid)
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
// row, one cell element per cell, every cell HTML-escaped, no inline styles — so
// it composes with any stylesheet (matching the goldmark extension's output).
// The rows a sheet declares with `#.header` are written as <th>, and the block
// of them at the top becomes a <thead>, which is the structure the declaration
// exists to express.
func writeHTML(w io.Writer, seen projection) error {
	rows := make([]string, len(seen.grid))
	for i, row := range seen.grid {
		rows[i] = htmlRow(row, isHeaderRow(seen.headerRows[i+1]))
	}
	body := `<table class="tsvsheet">` + "\n" + htmlSections(rows, seen.leadingHeaderRows()) + "</table>\n"
	return rendering(body).write(w)
}

// htmlSections wraps a leading header block in <thead> and the rest in <tbody>,
// leaving an undeclared table as the bare rows it has always been.
func htmlSections(rows []string, leading headerBlock) string {
	if leading == 0 {
		return strings.Join(rows, "")
	}
	head := "<thead>\n" + strings.Join(rows[:leading], "") + "</thead>\n"
	if int(leading) == len(rows) {
		return head
	}
	return head + "<tbody>\n" + strings.Join(rows[leading:], "") + "</tbody>\n"
}

// htmlRow renders one grid row as a <tr>, using <th> for a declared header row
// and <td> otherwise.
func htmlRow(row []string, isHeader isHeaderRow) string {
	tag := "td"
	if isHeader {
		tag = "th"
	}
	cells := make([]string, len(row))
	for i, cell := range row {
		cells[i] = "<" + tag + ">" + html.EscapeString(cell) + "</" + tag + ">"
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
