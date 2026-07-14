package serve

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandlerEndpoints(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		endpoint     string
		expectedType string
		expectedBody string
	}{
		{name: "health", endpoint: "/health", expectedType: "application/json", expectedBody: `{"status":"ok"}`},
		{name: "root", endpoint: "/", expectedType: "text/plain", expectedBody: "Example Server\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, must := assert.New(t), require.New(t)

			server := httptest.NewServer(handler(testLogger()))
			t.Cleanup(server.Close)

			resp, err := http.Get(server.URL + tt.endpoint)
			must.NoError(err)
			t.Cleanup(func() { _ = resp.Body.Close() })

			body, err := io.ReadAll(resp.Body)
			must.NoError(err)

			want.Equal(http.StatusOK, resp.StatusCode)
			want.Equal(tt.expectedType, resp.Header.Get("Content-Type"))
			want.Equal(tt.expectedBody, string(body))
		})
	}
}
