// Package serve's structural-edit endpoint: the row/column insert, delete,
// duplicate and fill operations the browser editor issues, kept apart from the
// read/compute handlers because each one mutates the sheet.
package serve

import (
	"net/http"

	"github.com/tsvsheet/go-tsvsheet"
)

// structureOp names a structural edit: inserting or deleting a row or column,
// relative to a cell.
type (
	structureOp string
	// structureRequest is the POST /api/structure body: the op and the 0-based cell
	// it is relative to.
	structureRequest struct {
		Op  structureOp `json:"op"`
		Row int         `json:"row"`
		Col int         `json:"col"`
	}
)

// handleStructure applies a row/column insert or delete and returns the new
// state; an unknown op, or a negative row/col, is a 400. The negative-index
// guard is defense in depth: the engine's InsertRow/InsertCol clamp the upper
// bound but a released engine panics on a negative index, so an untrusted
// request must be rejected at the boundary before it reaches the session.
func (srv Server) handleStructure(w http.ResponseWriter, r *http.Request) {
	var req structureRequest
	if !decode(w, r, &req) {
		return
	}
	if req.Row < 0 || req.Col < 0 {
		writeError(
			w,
			http.StatusBadRequest,
			tsvsheet.ErrInvalidValue.With(nil, "cell", "row and col must be non-negative"),
		)
		return
	}
	if err := srv.structuralEdit(req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, srv.session.Snapshot())
}

// structuralEdit applies one structural request, or reports ErrInvalidValue
// for an operation this server does not implement. It is a function rather
// than inline handler code so the error a caller receives can be matched by
// identity in a test, not merely recognised as a 400.
func (srv Server) structuralEdit(req structureRequest) error {
	if !srv.applyStructure(req.Op, tsvsheet.Address{Row: req.Row, Col: req.Col}) {
		return tsvsheet.ErrInvalidValue.With(nil, "op", string(req.Op))
	}
	return nil
}

// applyStructure dispatches a structural op to the session; the boolean reports
// whether the op was recognised.
func (srv Server) applyStructure(op structureOp, at tsvsheet.Address) bool {
	switch op {
	case opInsertRow:
		srv.session.InsertRow(at)
	case opDeleteRow:
		srv.session.DeleteRow(at)
	case opInsertCol:
		srv.session.InsertCol(at)
	case opDeleteCol:
		srv.session.DeleteCol(at)
	case opDuplicateRow:
		srv.session.DuplicateRow(at)
	case opDuplicateCol:
		srv.session.DuplicateCol(at)
	case opFillDown, opFillRight:
		srv.fillFromNeighbor(op, at)
	default:
		return false
	}
	return true
}

// fillFromNeighbor applies the single-cell fill ops: fill-down copies the cell
// above the selection into it, fill-right the cell to its left — Excel's
// Ctrl+D/Ctrl+R on a single cell. A selection on the top row (or leftmost
// column) has no source neighbor and is a no-op, as in Excel.
func (srv Server) fillFromNeighbor(op structureOp, at tsvsheet.Address) {
	from := tsvsheet.Address{Row: at.Row - 1, Col: at.Col}
	switch op {
	case opFillRight:
		from = tsvsheet.Address{Row: at.Row, Col: at.Col - 1}
	default:
	}
	if from.Row < 0 || from.Col < 0 {
		return
	}
	srv.session.Fill(from, tsvsheet.Span{From: at, To: at})
}
