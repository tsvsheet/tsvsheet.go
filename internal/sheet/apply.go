package sheet

import "github.com/uplang/tsvsheet.go/internal/tsvt"

// applyLine applies one body/final line at the given row (noRow in final).
// After partition, a section holds only Structural commands and cell Rows.
func (c *computation) applyLine(line tsvt.Line, row int) {
	if structural, ok := line.(tsvt.Structural); ok {
		c.applyStructural(structural)
		return
	}
	c.applyRow(line.(tsvt.Row), row)
}

// applyRow applies each cell of a row at its field index (§ ADR 0003 rule 17).
func (c *computation) applyRow(row tsvt.Row, dataRow int) {
	for field, cell := range row.Cells {
		c.applyCell(cell, field, dataRow)
	}
}

// applyCell writes one cell: a positional formula/literal targets its field
// index; a placement targets its referenced cell (§ ADR 0003 rule 17).
func (c *computation) applyCell(cell tsvt.Cell, field, dataRow int) {
	switch cl := cell.(type) {
	case tsvt.FormulaCell:
		c.writeAt(dataRow, field, c.resolverAt(dataRow).eval(cl.Expr).String())
	case tsvt.LiteralCell:
		c.writeAt(dataRow, field, cl.Value.Text)
	case tsvt.PlacementCell:
		c.applyPlacement(cl, dataRow)
	}
}

// writeAt writes a value at a resolved position, skipping the no-row context
// (a positional cell in the final phase has no row to target).
func (c *computation) writeAt(row, col int, s string) {
	if row == noRow {
		return
	}
	c.set(row, col, s)
}

// applyPlacement writes an addressed placement. A placement with no payload
// (a bare reference or header label) writes nothing; a row-selector target
// (no column) writes nothing (ADR 0003 rule 17).
func (c *computation) applyPlacement(cell tsvt.PlacementCell, dataRow int) {
	if cell.Payload == nil {
		return
	}
	row, col, ok := c.target(cell.Ref, dataRow)
	if !ok {
		return
	}
	c.set(row, col, c.payloadText(cell.Payload, dataRow))
}

// target resolves a placement reference to a concrete (row, col). A RowAll row
// means the current data row (the pass already iterates per row); a relative
// row with no current row, or a non-cell reference, is not a valid target.
func (c *computation) target(ref tsvt.Reference, dataRow int) (int, int, bool) {
	cell, ok := placementCell(ref)
	if !ok {
		return 0, 0, false
	}
	res := c.resolverAt(dataRow)
	cr := res.resolveColumn(cell.Col)
	if !cr.ok {
		return 0, 0, false
	}
	return targetRow(res, cell.Row, dataRow, cr.index)
}

// targetRow resolves a placement's row: an elided or all-rows row is the
// current data row; otherwise the resolved row, which must exist in the no-row
// context only if absolute.
func targetRow(res resolver, row tsvt.RowRef, dataRow, col int) (int, int, bool) {
	if row == nil || isAllRows(row) {
		if dataRow == noRow {
			return 0, 0, false
		}
		return dataRow, col, true
	}
	rowIdx, ok := res.resolveRow(row)
	if !ok || rowIdx < 0 {
		return 0, 0, false
	}
	return rowIdx, col, true
}

// isAllRows reports whether a row reference is the all-rows wildcard.
func isAllRows(row tsvt.RowRef) bool {
	_, ok := row.(tsvt.RowAll)
	return ok
}

// placementCell narrows a reference to the single cell endpoint a placement
// targets; ranges, grouped ranges, and row selectors are not placement targets.
func placementCell(ref tsvt.Reference) (tsvt.CellEndpoint, bool) {
	rangeRef, ok := ref.(tsvt.RangeRef)
	if !ok || rangeRef.To != nil {
		return tsvt.CellEndpoint{}, false
	}
	cell, ok := rangeRef.From.(tsvt.CellEndpoint)
	return cell, ok
}

// payloadText computes a placement's payload string: a formula is evaluated, a
// literal is verbatim.
func (c *computation) payloadText(payload tsvt.Payload, dataRow int) string {
	if formula, ok := payload.(tsvt.FormulaPayload); ok {
		return c.resolverAt(dataRow).eval(formula.Expr).String()
	}
	return payload.(tsvt.LiteralPayload).Value.Text
}

// resolverAt builds a resolver over the current grid at the given row.
func (c *computation) resolverAt(row int) resolver {
	return resolver{grid: c.grid, row: row, width: c.width, names: c.names}
}

// set writes a value at (row, col), growing the grid with empty rows/cells as
// needed so appends (row = last+1) and new columns are created.
func (c *computation) set(row, col int, s string) {
	for c.grid.rows() <= row {
		c.grid = append(c.grid, []string{})
	}
	line := c.grid[row]
	for len(line) <= col {
		line = append(line, "")
	}
	line[col] = s
	c.grid[row] = line
}
