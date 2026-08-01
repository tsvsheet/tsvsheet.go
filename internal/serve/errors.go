// What this server tells a client when it refuses: the sentinel each refusal
// carries, so a caller distinguishes one from another by identity rather than
// by reading a status code and guessing which rule produced it.
package serve

import (
	"encoding/json"
	"net/http"
)

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
