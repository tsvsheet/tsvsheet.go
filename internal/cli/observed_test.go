package cli

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestObservedLogsAndCountsEveryRequest pins R17 at this binary's shared
// wrapper: a served request produces one structured record with the fields an
// operator needs, and the counters move. Every serving surface here goes
// through this one function, so proving it once proves it for all of them.
func TestObservedLogsAndCountsEveryRequest(t *testing.T) {
	logs := &bytes.Buffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("body"))
	})
	rec := httptest.NewRecorder()
	observed(inner, "test_surface").ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sheet.tsvt", nil))

	require.Equal(t, http.StatusTeapot, rec.Code)
	var record map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(logs.String())), &record))
	assert.Equal(t, "request served", record["msg"])
	assert.Equal(t, "GET", record["method"])
	assert.Equal(t, "/sheet.tsvt", record["path"])
	assert.InDelta(t, http.StatusTeapot, record["status"], 0)
	assert.Equal(t, "WARN", record["level"], "a 4xx is worth an operator's attention")
	assert.InDelta(t, 4, record["bytes"], 0)
}
