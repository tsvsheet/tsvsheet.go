//go:build js && wasm

// Command wasm runs the tsvsheet engine entirely in the browser (no server),
// backing the docs playground. It registers a global `tsvsheet` object whose
// functions a page's JavaScript calls; each returns a JSON string (or a
// {"error": …} object). File-backed references (SHEET(…), "file"!A1) resolve to
// #REF! — there is no filesystem in the browser — but every other function,
// including the clock functions TODAY/NOW/ISNOW, works.
package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/uplang/tsvsheet.go/internal/constants"
	"github.com/uplang/tsvsheet.go/internal/session"
	"github.com/uplang/tsvsheet.go/internal/sheet"
)

// state is the one in-browser editing session.
var state *session.Session

func main() {
	state, _ = session.New([]byte("")) // start empty; load() replaces it

	obj := js.Global().Get("Object").New()
	obj.Set("load", js.FuncOf(load))
	obj.Set("setCell", js.FuncOf(setCell))
	obj.Set("structure", js.FuncOf(structure))
	obj.Set("references", js.FuncOf(references))
	obj.Set("explain", js.FuncOf(explain))
	obj.Set("recompute", js.FuncOf(recompute))
	obj.Set("source", js.FuncOf(source))
	js.Global().Set("tsvsheet", obj)

	select {} // keep the Go runtime alive for the callbacks
}

// snapshotJSON marshals the current read model.
func snapshotJSON() any {
	b, _ := json.Marshal(state.Snapshot())
	return string(b)
}

// errJSON marshals an error as {"error": …}.
func errJSON(err error) any {
	b, _ := json.Marshal(map[string]string{"error": err.Error()})
	return string(b)
}

// load parses a whole .tsvt document into the session, returning the new state
// or an {"error"} on a formula syntax error.
func load(_ js.Value, args []js.Value) any {
	s, err := session.New([]byte(args[0].String()))
	if err != nil {
		return errJSON(err)
	}
	state = s
	return snapshotJSON()
}

// setCell edits one cell's source (a literal or an =formula) and returns the
// recomputed state, or an {"error"} on a malformed formula.
func setCell(_ js.Value, args []js.Value) any {
	if err := state.SetCell(sheet.Address{Row: args[0].Int(), Col: args[1].Int()}, args[2].String()); err != nil {
		return errJSON(err)
	}
	return snapshotJSON()
}

// structure applies a row/column insert or delete relative to a cell.
func structure(_ js.Value, args []js.Value) any {
	at := sheet.Address{Row: args[1].Int(), Col: args[2].Int()}
	switch args[0].String() {
	case "insert-row":
		state.InsertRow(at)
	case "delete-row":
		state.DeleteRow(at)
	case "insert-col":
		state.InsertCol(at)
	case "delete-col":
		state.DeleteCol(at)
	default:
		return errJSON(constants.ErrInvalidValue.With(nil, "op", args[0].String()))
	}
	return snapshotJSON()
}

// references returns the selected cell's precedents and dependents.
func references(_ js.Value, args []js.Value) any {
	at, err := sheet.ParseAddress(sheet.AddressText(args[0].String()))
	if err != nil {
		return errJSON(err)
	}
	precedents, dependents := state.References(at)
	b, _ := json.Marshal(map[string]any{"precedents": precedents, "dependents": dependents})
	return string(b)
}

// explain traces how the cell at the given A1 address was produced.
func explain(_ js.Value, args []js.Value) any {
	at, err := sheet.ParseAddress(sheet.AddressText(args[0].String()))
	if err != nil {
		return errJSON(err)
	}
	trace, err := state.Explain(at)
	if err != nil {
		return errJSON(err)
	}
	b, _ := json.Marshal(trace)
	return string(b)
}

// recompute re-evaluates against the current clock (refreshing TODAY/NOW/ISNOW).
func recompute(_ js.Value, _ []js.Value) any {
	state.Recompute()
	return snapshotJSON()
}

// source returns the current spreadsheet as .tsvt text — for the live TSV pane
// and copy-to-clipboard.
func source(_ js.Value, _ []js.Value) any {
	return string(state.Source())
}
