//go:build integration

// The 018 serve integration angles: both servers driven through their full
// CLI composition over real HTTP — the browser-editor server round-trips an
// edit to disk, and the document API serves, conditions, and refuses over
// live requests, budgets included.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"
)

// TestIntegration_ServeSheetRoundTripsAnEditToDisk drives the browser-editor
// server end to end: load the file through loadServer, GET the computed
// state, PUT a cell edit, POST a save, and observe the edit ON DISK — the
// full read-edit-persist loop over real HTTP.
func TestIntegration_ServeSheetRoundTripsAnEditToDisk(t *testing.T) {
	t.Parallel()

	path := writeTemp(t, "sheet.tsvt", "1\t=A1*7\n")
	server, err := loadServer(serveConfig{source: sourcePath(path), limits: tsvsheet.DefaultLimits()})
	require.NoError(t, err)
	web := httptest.NewServer(server.Handler())
	t.Cleanup(web.Close)

	state, err := web.Client().Get(web.URL + "/api/state")
	require.NoError(t, err)
	t.Cleanup(func() { _ = state.Body.Close() })
	var got struct {
		Computed [][]string `json:"computed"`
	}
	require.NoError(t, json.NewDecoder(state.Body).Decode(&got))
	require.NotEmpty(t, got.Computed)
	assert.Equal(t, "7", got.Computed[0][1], "the formula served computed")

	edit, err := http.NewRequestWithContext(context.Background(), http.MethodPut, web.URL+"/api/cell",
		strings.NewReader(`{"row":0,"col":0,"text":"3"}`))
	require.NoError(t, err)
	putResp, err := web.Client().Do(edit)
	require.NoError(t, err)
	_ = putResp.Body.Close()
	require.Equal(t, http.StatusOK, putResp.StatusCode)

	saveResp, err := web.Client().Post(web.URL+"/api/save", "application/json", bytes.NewReader(nil))
	require.NoError(t, err)
	_ = saveResp.Body.Close()
	require.Equal(t, http.StatusOK, saveResp.StatusCode)

	onDisk, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "3\t=A1*7\n", string(onDisk), "the edit persisted to the file")
}

// TestIntegration_ServeAPIHonorsBudgetsOverHTTP drives `tsv serve api`
// through the full CLI (flag parsing, store, handler) with the listener
// stubbed, then exercises the captured server over live HTTP: an in-budget
// document reads and conditionally replaces; under a two-cell budget the
// same six-cell document answers 413 document-too-large, and an over-budget
// PUT is refused — the wire half of the bounded-store contract.
func TestIntegration_ServeAPIHonorsBudgetsOverHTTP(t *testing.T) {
	built := withServeStub(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(dir+"/six.tsvt", []byte(sixCells), 0o600))

	_, err := runCLI(t, "serve", "api", "--root", dir)
	require.NoError(t, err)
	require.Len(t, *built, 1)
	open := httptest.NewServer((*built)[0].Handler)
	t.Cleanup(open.Close)

	doc, err := open.Client().Get(open.URL + "/six.tsvt")
	require.NoError(t, err)
	defer func() { _ = doc.Body.Close() }()
	body, rev := readAllAndRev(t, doc)
	assert.Equal(t, sixCells, body, "the stored document serves verbatim")
	require.NotEmpty(t, rev)

	replace, err := http.NewRequestWithContext(context.Background(), http.MethodPut, open.URL+"/six.tsvt",
		strings.NewReader("9\t8\n"))
	require.NoError(t, err)
	replace.Header.Set("If-Match", `"`+rev+`"`)
	replace.Header.Set("Content-Type", "application/vnd.tsvsheet.doc+tsv")
	putResp, err := open.Client().Do(replace)
	require.NoError(t, err)
	_ = putResp.Body.Close()
	require.Equal(t, http.StatusOK, putResp.StatusCode, "a correctly-conditioned replace lands")

	_, err = runCLI(t, "serve", "api", "--root", dir, "--max-cells", "2")
	require.NoError(t, err)
	require.Len(t, *built, 2)
	tight := httptest.NewServer((*built)[1].Handler)
	t.Cleanup(tight.Close)

	require.NoError(t, os.WriteFile(dir+"/six.tsvt", []byte(sixCells), 0o600))
	refused, err := tight.Client().Get(tight.URL + "/six.tsvt")
	require.NoError(t, err)
	defer func() { _ = refused.Body.Close() }()
	refusedBody, _ := readAllAndRev(t, refused)
	assert.Equal(t, http.StatusRequestEntityTooLarge, refused.StatusCode,
		"an over-budget stored document answers 413")
	assert.Contains(t, refusedBody, "document-too-large", "the problem names its type")

	create, err := http.NewRequestWithContext(context.Background(), http.MethodPut, tight.URL+"/new.tsvt",
		strings.NewReader(sixCells))
	require.NoError(t, err)
	create.Header.Set("If-None-Match", "*")
	create.Header.Set("Content-Type", "application/vnd.tsvsheet.doc+tsv")
	refusedPut, err := tight.Client().Do(create)
	require.NoError(t, err)
	_ = refusedPut.Body.Close()
	assert.Equal(t, http.StatusRequestEntityTooLarge, refusedPut.StatusCode,
		"a body the server could never load back is refused")
}

// readAllAndRev drains a response (the caller owns closing it), returning
// its body and ETag revision.
func readAllAndRev(t *testing.T, resp *http.Response) (string, string) {
	t.Helper()
	var buf bytes.Buffer
	_, err := buf.ReadFrom(resp.Body)
	require.NoError(t, err)
	return buf.String(), strings.Trim(resp.Header.Get("ETag"), `"`)
}
