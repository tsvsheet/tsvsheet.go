package cli

import (
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

// hiddenPolicy decides what render does with the rows and columns a sheet's
// `#.hide` directives declare. Keeping them is the default and the only safe
// one for a pipeline: a hidden row is still data, and silently dropping it
// would make `tsv render` lose values a reader never asked it to lose.
type hiddenPolicy string

// The policies --hidden accepts.
const (
	hiddenKeep hiddenPolicy = "keep"
	hiddenDrop hiddenPolicy = "drop"
)

// projection is a computed grid as a viewport sees it, with the header rows
// re-numbered to wherever they ended up once hidden rows were dropped.
type projection struct {
	headerRows tsvsheet.Selection
	grid       tsvsheet.Grid
}

// project applies a view to a computed grid. Keeping hidden cells leaves the
// grid exactly as computed — the declarations are advisory to a viewport, not
// to a pipeline — while dropping them yields the projected artifact, which is a
// different thing from the sheet and is why it takes an explicit flag.
func project(grid tsvsheet.Grid, view tsvsheet.View, policy hiddenPolicy) (projection, error) {
	switch policy {
	case hiddenKeep:
		return projection{grid: grid, headerRows: view.HeaderRows}, nil
	case hiddenDrop:
		return dropHidden(grid, view), nil
	}
	return projection{}, constants.ErrUnknownHiddenPolicy.With(nil, "hidden", string(policy))
}

// dropHidden removes the hidden rows and columns, tracking where each surviving
// header row landed so a format that marks headers still marks the right ones.
func dropHidden(grid tsvsheet.Grid, view tsvsheet.View) projection {
	out := projection{grid: make(tsvsheet.Grid, 0, len(grid)), headerRows: tsvsheet.Selection{}}
	for r, row := range grid {
		at := r + 1
		if view.HiddenRows[at] {
			continue
		}
		if view.HeaderRows[at] {
			out.headerRows[len(out.grid)+1] = true
		}
		out.grid = append(out.grid, keepColumns(row, view.HiddenCols))
	}
	return out
}

// keepColumns drops the hidden columns from one row.
func keepColumns(row []string, hidden tsvsheet.Selection) []string {
	kept := make([]string, 0, len(row))
	for c, cell := range row {
		if !hidden[c+1] {
			kept = append(kept, cell)
		}
	}
	return kept
}

// isHeaderRow marks a row the sheet declared as a header, which decides whether
// its cells are written as <th>.
type isHeaderRow bool

// headerBlock is how many rows of a leading header block a table has — the part
// an HTML table can carry in a <thead>.
type headerBlock int

// leadingHeaderRows counts the header rows that form a block at the top. A
// header declared further down still gets header cells, in place, since pulling
// it into a <thead> would reorder the table.
func (p projection) leadingHeaderRows() headerBlock {
	n := headerBlock(0)
	for p.headerRows[int(n)+1] {
		n++
	}
	return n
}
