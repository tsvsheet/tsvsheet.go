package cli

import (
	"io"

	"github.com/uplang/tsvsheet.go/internal/sheet"
	"github.com/uplang/tsvsheet.go/internal/tsvt"
)

// parseTemplate reads a template source fully and parses it to an AST.
func parseTemplate(r io.Reader) (tsvt.Template, error) {
	src, err := readAll(r)
	if err != nil {
		return tsvt.Template{}, err
	}
	return tsvt.Parse(tsvt.Source(src))
}

// computeWorksheet parses the template, reads the data grid, and computes the
// output grid.
func computeWorksheet(templateReader, dataReader io.Reader) (sheet.Grid, error) {
	tmpl, err := parseTemplate(templateReader)
	if err != nil {
		return nil, err
	}
	grid, err := sheet.ReadTSV(dataReader)
	if err != nil {
		return nil, err
	}
	return sheet.Compute(tmpl, grid)
}
