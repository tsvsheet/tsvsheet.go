package importer

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestSchemeAllowed(t *testing.T) {
	t.Parallel()

	// https is allowed for any host; http only for a loopback target; any other
	// scheme is rejected regardless of host.
	assert.True(t, schemeAllowed("https", "example.com"))
	assert.True(t, schemeAllowed("https", "127.0.0.1"))
	assert.True(t, schemeAllowed("http", "127.0.0.1"))
	assert.True(t, schemeAllowed("http", "localhost"))
	assert.False(t, schemeAllowed("http", "example.com"))
	assert.False(t, schemeAllowed("ftp", "127.0.0.1"))
	assert.False(t, schemeAllowed("file", "localhost"))
}

func TestIsLoopback(t *testing.T) {
	t.Parallel()

	cases := map[Host]bool{
		"localhost":   true,  // the loopback name
		"LocalHost":   true,  // case-insensitive
		"127.0.0.1":   true,  // IPv4 loopback
		"127.0.0.5":   true,  // anywhere in 127.0.0.0/8
		"::1":         true,  // IPv6 loopback
		"example.com": false, // a name that is not localhost
		"8.8.8.8":     false, // a routable IP
		"":            false, // empty is not loopback
	}
	for host, want := range cases {
		assert.Equal(t, want, IsLoopback(host), string(host))
	}
}

func TestHostAllowed_EmptyDeniesAll(t *testing.T) {
	t.Parallel()

	f := Fetcher{}
	assert.False(t, f.hostAllowed("example.com"))
}
