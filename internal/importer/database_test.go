package importer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

func TestNewDataBase_SchemePolicyHasNoExemption(t *testing.T) {
	t.Parallel()

	cases := map[string]error{
		"https://data.example.com/team/": nil,                       // https anywhere
		"http://127.0.0.1:8137/":         nil,                       // http to loopback
		"http://localhost:8137/":         nil,                       // loopback by name
		"http://data.example.com/team/":  constants.ErrImportScheme, // cleartext to a remote host
		"ftp://data.example.com/":        constants.ErrImportScheme, // not http(s)
		"data/balances.tsv":              constants.ErrImportScheme, // no scheme at all
	}
	for raw, want := range cases {
		_, err := NewDataBase(raw)
		if want == nil {
			require.NoError(t, err, raw)
			continue
		}
		require.ErrorIs(t, err, want, raw)
	}
}

func TestNewDataBase_MalformedURL(t *testing.T) {
	t.Parallel()

	_, err := NewDataBase("https://exa mple.com/\x7f")
	require.ErrorIs(t, err, constants.ErrImportURL)
}

func TestBasePrefix_NormalizesToTrailingSlash(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"":           "/",
		"/":          "/",
		"/team":      "/team/",
		"/team/":     "/team/",
		"/a/../team": "/team/",
		"/team/sub/": "/team/sub/",
	}
	for in, want := range cases {
		assert.Equal(t, want, basePrefix(in), in)
	}
}

func TestFetcherResolve_AbsolutePassesThroughUnchanged(t *testing.T) {
	t.Parallel()

	f := New(Config{})
	got, fromBase, err := f.resolve("https://example.org/series.tsv")
	require.NoError(t, err)
	assert.Equal(t, "https://example.org/series.tsv", got.String())
	assert.False(t, bool(fromBase), "an absolute URL is not a base reference and stays under the allowlist")
}

func TestFetcherResolve_RelativeResolvesUnderTheBase(t *testing.T) {
	t.Parallel()

	base, err := NewDataBase("https://data.example.com/team")
	require.NoError(t, err)
	f := New(Config{Base: base})

	cases := map[tsvsheet.ImportURL]string{
		"balances.tsv":   "https://data.example.com/team/balances.tsv",
		"macro/nfci.tsv": "https://data.example.com/team/macro/nfci.tsv",
		"./balances.tsv": "https://data.example.com/team/balances.tsv",
		"sub/../x.tsv":   "https://data.example.com/team/x.tsv",
	}
	for ref, want := range cases {
		got, fromBase, err := f.resolve(ref)
		require.NoError(t, err, string(ref))
		assert.Equal(t, want, got.String(), string(ref))
		assert.True(t, bool(fromBase), string(ref))
	}
}

func TestFetcherResolve_TraversalAboveTheBaseIsRefused(t *testing.T) {
	t.Parallel()

	base, err := NewDataBase("https://data.example.com/team/")
	require.NoError(t, err)
	f := New(Config{Base: base})

	// Every shape of climb-out, including the escaped form and the sibling whose
	// name merely starts the same way.
	for _, ref := range []tsvsheet.ImportURL{
		"../../admin/keys.tsv",
		"../admin/keys.tsv",
		"sub/../../../etc/passwd",
		"%2e%2e/admin/keys.tsv",
		"/etc/passwd",
	} {
		_, _, err := f.resolve(ref)
		require.ErrorIs(t, err, constants.ErrImportEscape, string(ref))
	}
}

func TestFetcherResolve_SiblingPrefixIsNotUnderTheBase(t *testing.T) {
	t.Parallel()

	base, err := NewDataBase("https://data.example.com/team/")
	require.NoError(t, err)
	f := New(Config{Base: base})

	_, _, err = f.resolve("../teamster/secrets.tsv")
	require.ErrorIs(t, err, constants.ErrImportEscape)
}

func TestFetcherResolve_SchemeRelativeReferenceCannotRetargetTheHost(t *testing.T) {
	t.Parallel()

	base, err := NewDataBase("https://data.example.com/team/")
	require.NoError(t, err)
	f := New(Config{Base: base})

	// url.Parse reports "//elsewhere/x" as relative (no scheme), so resolving it
	// would point at elsewhere.example — the one way a sheet could choose its
	// own server. It must be refused, not resolved.
	_, _, err = f.resolve("//elsewhere.example/x.tsv")
	require.ErrorIs(t, err, constants.ErrImportURL)
}

func TestFetcherResolve_RelativeWithNoBaseIsRefused(t *testing.T) {
	t.Parallel()

	f := New(Config{})
	_, _, err := f.resolve("balances.tsv")
	require.ErrorIs(t, err, constants.ErrImportNoBase)
}

func TestFetcherResolve_MalformedReference(t *testing.T) {
	t.Parallel()

	f := New(Config{})
	_, _, err := f.resolve("://nonsense")
	require.ErrorIs(t, err, constants.ErrImportURL)
}
