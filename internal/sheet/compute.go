package sheet

import "github.com/uplang/tsvsheet.go/internal/tsvt"

// phase groups a template's lines into the SPECIFICATION §9 sections.
type phase struct {
	header []tsvt.Row
	body   []tsvt.Line
	final  []tsvt.Line
}

// computation carries the mutable computed grid, bound header names, and the
// logical column width through a single processing pass.
type computation struct {
	grid  Grid
	names map[string]int
	width int
}

// Compute runs the SPECIFICATION §9 processing model over a data grid and a
// parsed template, returning the computed output grid. Cell-level failures are
// error values in the grid (ADR 0003); the error return is reserved for a
// template the processor structurally rejects.
func Compute(t tsvt.Template, g Grid) (Grid, error) {
	for _, d := range Check(t) {
		if d.Fatal {
			return nil, d.err()
		}
	}
	phases := partition(t)
	comp := &computation{grid: g.clone(), names: map[string]int{}, width: g.cols()}
	comp.bindHeaders(phases.header)
	comp.runBody(phases.body)
	comp.runFinal(phases.final)
	return comp.grid, nil
}

// partition splits template lines into header / body / final by section markers
// (§4). With no markers every non-marker line is body (§4 minimal form).
func partition(t tsvt.Template) phase {
	var (
		p       phase
		section = &p.body
		headers = 0
	)
	for _, line := range t.Lines {
		section, headers = route(line, &p, section, headers)
	}
	return p
}

// route dispatches one template line to the active section, honoring a pending
// header-line count.
func route(line tsvt.Line, p *phase, section *[]tsvt.Line, headers int) (*[]tsvt.Line, int) {
	if headers > 0 {
		if row, ok := line.(tsvt.Row); ok {
			p.header = append(p.header, row)
		}
		return section, headers - 1
	}
	switch m := line.(type) {
	case tsvt.HeaderMarker:
		return section, m.Count
	case tsvt.BodyMarker:
		return &p.body, 0
	case tsvt.FinalMarker:
		return &p.final, 0
	default:
		*section = append(*section, line)
		return section, 0
	}
}

// bindHeaders names columns from the header rows: each row's cells name columns
// left to right by field index (§5.1).
func (c *computation) bindHeaders(rows []tsvt.Row) {
	for _, row := range rows {
		for col, cell := range row.Cells {
			if name, ok := headerName(cell); ok {
				c.names[name] = col
			}
		}
	}
}

// headerName extracts a column name from a header cell: a bareword/number
// literal, or a named-column reference. Other cells (formulas, empties) do not
// name a column.
func headerName(cell tsvt.Cell) (string, bool) {
	switch c := cell.(type) {
	case tsvt.LiteralCell:
		return c.Value.Text, true
	case tsvt.PlacementCell:
		if named, ok := c.Ref.(tsvt.RangeRef); ok {
			return namedFromRange(named)
		}
	}
	return "", false
}

// namedFromRange returns the label of a single named-column reference used as a
// header cell (`"Sum"`); other reference shapes do not name a column.
func namedFromRange(ref tsvt.RangeRef) (string, bool) {
	cell, ok := ref.From.(tsvt.CellEndpoint)
	if !ok {
		return "", false
	}
	named, ok := cell.Col.(tsvt.ColNamed)
	if !ok {
		return "", false
	}
	return named.Name, true
}

// runBody applies each body line to every data row in order (§9.4).
func (c *computation) runBody(lines []tsvt.Line) {
	for row := 0; row < c.grid.rows(); row++ {
		for _, line := range lines {
			c.applyLine(line, row)
		}
	}
}

// runFinal applies each final line once over the finished grid (§9.5), with no
// current row.
func (c *computation) runFinal(lines []tsvt.Line) {
	for _, line := range lines {
		c.applyLine(line, noRow)
	}
}

// noRow marks the absence of a current row (final phase).
const noRow = -1
