package tsvt

import (
	grammar "github.com/uplang/tsvsheet.go/src/grammar/tsvsheet"
)

// buildReference builds an A1 reference: a single cell or a two-cell range.
func buildReference(ctx grammar.IReferenceContext) (Reference, error) {
	cells := ctx.AllCellRef()
	from, err := buildCellRef(cells[0])
	if err != nil {
		return nil, err
	}
	if len(cells) == 1 {
		return RangeRef{From: from}, nil
	}
	to, err := buildCellRef(cells[1])
	if err != nil {
		return nil, err
	}
	return RangeRef{From: from, To: &to}, nil
}

// buildCellRef builds one A1 cell (column letters + 1-based row).
func buildCellRef(ctx grammar.ICellRefContext) (CellRef, error) {
	row, err := intToken(ctx.NUMBER())
	if err != nil {
		return CellRef{}, err
	}
	return CellRef{Col: ctx.COL().GetText(), Row: row}, nil
}
