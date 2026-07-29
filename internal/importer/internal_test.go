package importer

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

func TestHostAllowed_FirstOfSeveral(t *testing.T) {
	t.Parallel()

	f := Fetcher{allowed: []HostPattern{"a.com", "*.b.com", "c.com"}}
	assert.True(t, f.hostAllowed("x.b.com")) // matched by the wildcard entry
	assert.False(t, f.hostAllowed("d.com"))  // exhausts the list
}

// errReader always fails, exercising readCapped's io.ReadAll error branch.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestReadCapped_ReadError(t *testing.T) {
	t.Parallel()

	f := Fetcher{maxBytes: 16}
	_, err := f.readCapped(errReader{})
	assert.ErrorIs(t, err, constants.ErrImportRead)
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

func TestNew_DefaultsClientAndInstallsCheckRedirect(t *testing.T) {
	t.Parallel()

	f := New(Config{})
	require.NotNil(t, f.client)               // nil Client → default built
	require.NotNil(t, f.client.CheckRedirect) // redirect guard installed
}

func TestNew_KeepsInjectedClient(t *testing.T) {
	t.Parallel()

	injected := &http.Client{}
	f := New(Config{Client: injected})
	assert.Same(t, injected, f.client)
	require.NotNil(t, injected.CheckRedirect) // installed onto the injected client
}
