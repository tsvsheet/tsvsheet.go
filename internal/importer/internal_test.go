package importer

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/constants"
)

func TestMatchHost_Wildcard(t *testing.T) {
	t.Parallel()

	const pat HostPattern = "*.example.com"
	cases := map[Host]bool{
		"a.example.com":   true,  // proper subdomain
		"x.y.example.com": true,  // deep subdomain
		"A.Example.CoM":   true,  // case-insensitive
		"example.com":     false, // apex is NOT matched by *.
		"evilexample.com": false, // lookalike: char before "example.com" is a letter, not a dot
		".example.com":    false, // bare-suffix trick: empty label
		"example.org":     false, // different domain
	}
	for host, want := range cases {
		assert.Equal(t, want, matchHost(pat, host), string(host))
	}
}

func TestMatchHost_Exact(t *testing.T) {
	t.Parallel()

	const pat HostPattern = "Example.COM"
	assert.True(t, matchHost(pat, "example.com"))    // case-insensitive exact
	assert.False(t, matchHost(pat, "a.example.com")) // exact does not match subdomains
	assert.False(t, matchHost(pat, "example.org"))
}

func TestHostAllowed_EmptyDeniesAll(t *testing.T) {
	t.Parallel()

	f := Fetcher{}
	assert.False(t, f.hostAllowed("example.com"))
}

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
