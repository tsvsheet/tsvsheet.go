// Package serve exposes a session.Session over an HTTP JSON API and hosts the
// embedded browser spreadsheet that consumes it. It is a thin projection: every
// endpoint round-trips through the one shared engine, so the web UI holds no
// language semantics.
package serve

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/uplang/go-tsvsheet"

	"github.com/uplang/tsvsheet.go/internal/constants"
	"github.com/uplang/tsvsheet.go/internal/refresh"
	"github.com/uplang/tsvsheet.go/internal/session"
)

// Saver persists the spreadsheet's current source. It is injected so the server
// stays filesystem-free and testable.
type Saver func() error

// Server serves the JSON API and embedded UI for one spreadsheet session.
type Server struct {
	session *session.Session
	save    Saver
	refresh refresh.Next
}

// NewServer builds a server over a session, a save function, and an auto-refresh
// cadence (nil disables periodic recomputation of volatile cells).
func NewServer(s *session.Session, save Saver, next refresh.Next) Server {
	return Server{session: s, save: save, refresh: next}
}

// nextRefreshMillis is the delay until the next auto-refresh, in milliseconds
// (0 = none) — evaluated now, so an isnow schedule reports its next occurrence.
func (srv Server) nextRefreshMillis() int {
	if srv.refresh == nil {
		return 0
	}
	d := srv.refresh(time.Now())
	if d <= 0 {
		return 0
	}
	return int(d.Milliseconds())
}

// Handler returns the HTTP handler: the JSON API under /api/ and the embedded
// single-page UI at the root.
func (srv Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/state", srv.handleState)
	mux.HandleFunc("GET /api/config", srv.handleConfig)
	mux.HandleFunc("PUT /api/cell", srv.handleCell)
	mux.HandleFunc("POST /api/save", srv.handleSave)
	mux.HandleFunc("POST /api/recompute", srv.handleRecompute)
	mux.HandleFunc("POST /api/refresh-imports", srv.handleRefreshImports)
	mux.HandleFunc("GET /api/explain", srv.handleExplain)
	mux.HandleFunc("GET /api/references", srv.handleReferences)
	mux.HandleFunc("POST /api/structure", srv.handleStructure)
	mux.HandleFunc("GET /api/embedded", srv.handleEmbedded)
	mux.Handle("GET /", uiHandler())
	return guardCSRF(mux)
}

// guardCSRF refuses cross-origin state-changing requests. serve is a local
// single-user editor whose saves write host files, so it binds loopback by
// default; but a page open in the operator's browser can still fire a "simple"
// cross-origin POST at it (a CSRF write it cannot read back). A browser stamps
// such a request Sec-Fetch-Site: cross-site|same-site, which script cannot
// forge, so mutating requests carrying it are refused; safe methods (GET/HEAD)
// and non-browser clients (no Sec-Fetch-Site, e.g. curl) pass through.
func guardCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isMutating := r.Method != http.MethodGet && r.Method != http.MethodHead
		site := r.Header.Get("Sec-Fetch-Site")
		if isMutating && (site == "cross-site" || site == "same-site") {
			writeError(w, http.StatusForbidden, constants.ErrForbidden.With(nil))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// configResponse is the GET /api/config body: the delay until the first
// auto-refresh in milliseconds (0 = none).
type configResponse struct {
	NextRefreshMillis int `json:"next_refresh_millis"`
}

// handleConfig returns the UI's initial configuration (the first refresh delay).
func (srv Server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, configResponse{NextRefreshMillis: srv.nextRefreshMillis()})
}

// recomputeResponse is the POST /api/recompute body: the refreshed read model
// plus the delay until the next refresh, so the UI reschedules itself.
type recomputeResponse struct {
	session.State
	NextRefreshMillis int `json:"next_refresh_millis"`
}

// handleRecompute re-evaluates the sheet against the current clock (refreshing
// volatile cells) and returns the new state and the next refresh delay.
func (srv Server) handleRecompute(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, recomputeResponse{
		State:             srv.session.Recompute(),
		NextRefreshMillis: srv.nextRefreshMillis(),
	})
}

// handleRefreshImports drops cached content-typed imports and recomputes,
// returning the refreshed read model. It is the explicit "refresh imports"
// action, distinct from POST /api/recompute (the clock refresh) — imports never
// ride the auto-refresh ticker (ADR 0006 §6).
func (srv Server) handleRefreshImports(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, srv.session.RefreshImports())
}

// handleState returns the current spreadsheet read model.
func (srv Server) handleState(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, srv.session.Snapshot())
}

// cellRequest is the PUT /api/cell body: row and col are 0-based grid indices,
// text is the cell's new source (a literal or an "=formula").
type cellRequest struct {
	Text string `json:"text"`
	Row  int    `json:"row"`
	Col  int    `json:"col"`
}

// handleCell edits one cell's source and returns the new state, or 422 with the
// formula syntax error.
func (srv Server) handleCell(w http.ResponseWriter, r *http.Request) {
	var req cellRequest
	if !decode(w, r, &req) {
		return
	}
	if err := srv.session.SetCell(tsvsheet.Address{Row: req.Row, Col: req.Col}, req.Text); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusOK, srv.session.Snapshot())
}

// saveResponse is the POST /api/save body.
type saveResponse struct {
	IsSaved bool `json:"saved"`
}

// handleSave persists the spreadsheet and clears the dirty flag.
func (srv Server) handleSave(w http.ResponseWriter, _ *http.Request) {
	if err := srv.save(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	srv.session.MarkSaved()
	writeJSON(w, http.StatusOK, saveResponse{IsSaved: true})
}

// handleExplain traces one cell named by the `cell` query parameter.
func (srv Server) handleExplain(w http.ResponseWriter, r *http.Request) {
	at, err := tsvsheet.ParseAddress(tsvsheet.AddressText(r.URL.Query().Get("cell")))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	trace, err := srv.session.Explain(at)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, trace)
}

// structureOp names a structural edit: inserting or deleting a row or column,
// relative to a cell.
type structureOp string

const (
	opInsertRow structureOp = "insert-row"
	opDeleteRow structureOp = "delete-row"
	opInsertCol structureOp = "insert-col"
	opDeleteCol structureOp = "delete-col"
)

// structureRequest is the POST /api/structure body: the op and the 0-based cell
// it is relative to.
type structureRequest struct {
	Op  structureOp `json:"op"`
	Row int         `json:"row"`
	Col int         `json:"col"`
}

// handleStructure applies a row/column insert or delete and returns the new
// state; an unknown op is a 400.
func (srv Server) handleStructure(w http.ResponseWriter, r *http.Request) {
	var req structureRequest
	if !decode(w, r, &req) {
		return
	}
	if !srv.applyStructure(req.Op, tsvsheet.Address{Row: req.Row, Col: req.Col}) {
		writeError(w, http.StatusBadRequest, tsvsheet.ErrInvalidValue.With(nil, "op", string(req.Op)))
		return
	}
	writeJSON(w, http.StatusOK, srv.session.Snapshot())
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
	default:
		return false
	}
	return true
}

// referencesResponse is the GET /api/references body: the selected cell's
// precedents (spans its formula reads) and dependents (cells that read it).
type referencesResponse struct {
	Precedents []tsvsheet.Span    `json:"precedents"`
	Dependents []tsvsheet.Address `json:"dependents"`
}

// handleReferences returns the dependency edges of the cell named by the `cell`
// query parameter, for highlighting. An off-grid or non-formula cell simply has
// empty edges — only a malformed address is a 400.
func (srv Server) handleReferences(w http.ResponseWriter, r *http.Request) {
	at, err := tsvsheet.ParseAddress(tsvsheet.AddressText(r.URL.Query().Get("cell")))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	precedents, dependents := srv.session.References(at)
	writeJSON(w, http.StatusOK, referencesResponse{Precedents: precedents, Dependents: dependents})
}

// embeddedResponse is the GET /api/embedded body: the sub-sheet a SHEET(...)
// cell embeds — its resolved path and its own computed grid.
type embeddedResponse struct {
	Path string        `json:"path"`
	Grid tsvsheet.Grid `json:"grid"`
}

// handleEmbedded returns the nested sub-sheet embedded by the cell named in the
// `cell` query parameter, or 404 when that cell is not a resolvable SHEET call.
func (srv Server) handleEmbedded(w http.ResponseWriter, r *http.Request) {
	at, err := tsvsheet.ParseAddress(tsvsheet.AddressText(r.URL.Query().Get("cell")))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	path, grid, ok := srv.session.Embedded(at)
	if !ok {
		writeError(w, http.StatusNotFound, tsvsheet.ErrNotFound.With(nil, "cell", at.String()))
		return
	}
	writeJSON(w, http.StatusOK, embeddedResponse{Path: string(path), Grid: grid})
}

// maxRequestBytes bounds a request body so a large or slow upload cannot
// exhaust memory; a cell edit is tiny and even a full sheet load fits easily.
const maxRequestBytes = 32 << 20 // 32 MiB

// decode reads a JSON request body into v, writing a 400 and returning false on
// a malformed or over-large body.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return false
	}
	return true
}

// errorResponse is the JSON error envelope.
type errorResponse struct {
	Error string `json:"error"`
}

// httpStatus is an HTTP response status code.
type httpStatus int

// writeError writes a JSON error envelope with the given status.
func writeError(w http.ResponseWriter, status httpStatus, err error) {
	writeJSON(w, status, errorResponse{Error: err.Error()})
}

// writeJSON encodes v as JSON with the given status.
func writeJSON(w http.ResponseWriter, status httpStatus, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(int(status))
	_ = json.NewEncoder(w).Encode(v)
}
