package importer_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
	"github.com/tsvsheet/tsvsheet.go/internal/importer"
)

// tabularServer answers every request with a TSV body, and records the paths it
// was asked for so a test can assert what the base resolved to.
func tabularServer(t *testing.T, seen *[]string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = append(*seen, r.URL.Path)
		w.Header().Set("Content-Type", "text/tab-separated-values")
		_, _ = w.Write([]byte("Brokerage\t310000\n"))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestFetch_RelativeReferenceResolvesAgainstTheBaseWithNoAllowlist(t *testing.T) {
	t.Parallel()

	var seen []string
	server := tabularServer(t, &seen)
	base, err := importer.NewDataBase(server.URL + "/team")
	require.NoError(t, err)

	// No AllowedHosts at all: naming the base is the authorization, and it must
	// not require handing out network permission via --import-host.
	fetcher := importer.New(importer.Config{Base: base, MaxBytes: 1 << 20})

	res, err := fetcher.Fetch("balances.tsv", "application/vnd.tsvsheet+tsv")
	require.NoError(t, err)
	assert.Equal(t, "Brokerage\t310000\n", string(res.Body))
	assert.Equal(t, []string{"/team/balances.tsv"}, seen, "resolved under the base, not beside it")
}

func TestFetch_AbsoluteURLStillNeedsTheAllowlistEvenWithABase(t *testing.T) {
	t.Parallel()

	var seen []string
	server := tabularServer(t, &seen)
	base, err := importer.NewDataBase(server.URL + "/team/")
	require.NoError(t, err)
	fetcher := importer.New(importer.Config{Base: base, MaxBytes: 1 << 20})

	// The very same host, written absolutely in the sheet, is refused: a base
	// authorizes the base, never a host.
	_, err = fetcher.Fetch(tsvsheet.ImportURL(server.URL+"/team/balances.tsv"), "application/vnd.tsvsheet+tsv")
	require.ErrorIs(t, err, constants.ErrImportHostDenied)
	assert.Empty(t, seen, "refused before any network I/O")
}

func TestFetch_RelativeReferenceWithNoBaseNeverReachesTheNetwork(t *testing.T) {
	t.Parallel()

	var seen []string
	_ = tabularServer(t, &seen)
	fetcher := importer.New(importer.Config{MaxBytes: 1 << 20})

	_, err := fetcher.Fetch("balances.tsv", "application/vnd.tsvsheet+tsv")
	require.ErrorIs(t, err, constants.ErrImportNoBase)
	assert.Empty(t, seen)
}

func TestFetch_TraversalAboveTheBaseNeverReachesTheNetwork(t *testing.T) {
	t.Parallel()

	var seen []string
	server := tabularServer(t, &seen)
	base, err := importer.NewDataBase(server.URL + "/team/")
	require.NoError(t, err)
	fetcher := importer.New(importer.Config{Base: base, MaxBytes: 1 << 20})

	_, err = fetcher.Fetch("../../admin/keys.tsv", "application/vnd.tsvsheet+tsv")
	require.ErrorIs(t, err, constants.ErrImportEscape)
	assert.Empty(t, seen, "the refusal happens before the request is built")
}

func TestFetch_UnparseableReferenceIsRejected(t *testing.T) {
	t.Parallel()

	base, err := importer.NewDataBase("http://127.0.0.1:1/")
	require.NoError(t, err)
	fetcher := importer.New(importer.Config{Base: base, MaxBytes: 1 << 20})

	// Resolves against the base, but is not a URL http.NewRequest will accept.
	_, err = fetcher.Fetch("\x7f\x00", "application/vnd.tsvsheet+tsv")
	require.ErrorIs(t, err, constants.ErrImportURL)
}

func TestLoopbackBase_ResolvesRelativeReferences(t *testing.T) {
	t.Parallel()

	var seen []string
	server := tabularServer(t, &seen)
	// The shape startScopedData uses: a base built from an address this process
	// just bound, never parsed from text.
	fetcher := importer.New(importer.Config{
		Base:     importer.LoopbackBase(server.Listener.Addr().String()),
		MaxBytes: 1 << 20,
	})

	res, err := fetcher.Fetch("balances.tsv", "application/vnd.tsvsheet+tsv")
	require.NoError(t, err)
	assert.Equal(t, "Brokerage\t310000\n", string(res.Body))
	assert.Equal(t, []string{"/balances.tsv"}, seen)
}
