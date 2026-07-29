// Package importer's policy half: which hosts an absolute IMPORT* may reach and
// over which transport. This is the network security boundary's decision layer —
// the allowlist an operator supplies, and the scheme rule that admits plain http
// only to the local machine.
package importer

import (
	"net"
	"strings"
)

// urlScheme is a request URL's scheme ("https" or "http"), checked by the scheme
// policy against the target host.
type urlScheme string

// Host is a request URL's hostname (port and IPv6 brackets already stripped),
// checked against the allowlist.
type Host string

// HostPattern is one allowlist entry: an exact host ("example.com") or a
// leading-"*." wildcard ("*.example.com") that matches any proper subdomain but
// never the apex.
type HostPattern string

// hostAllowed reports whether host matches any allowlist entry; an empty
// allowlist denies everything.
func (f Fetcher) hostAllowed(host Host) bool {
	for _, pattern := range f.allowed {
		if matchHost(pattern, host) {
			return true
		}
	}
	return false
}

// matchHost reports whether host satisfies pattern, case-insensitively: a
// leading "*." is a subdomain wildcard, anything else is an exact host.
func matchHost(pattern HostPattern, host Host) bool {
	p := strings.ToLower(string(pattern))
	h := strings.ToLower(string(host))
	if suffix, ok := strings.CutPrefix(p, "*."); ok {
		return wildcardMatch(Host(suffix), Host(h))
	}
	return p == h
}

// wildcardMatch reports whether host is a proper subdomain of suffix: host must
// end with "."+suffix AND carry a non-empty label before it. This rejects the
// apex ("example.com" does not end with ".example.com"), the lookalike
// ("evilexample.com" — the char before "example.com" is a letter, not a dot),
// and the bare-suffix trick (".example.com" — the label before the dot is
// empty).
func wildcardMatch(suffix, host Host) bool {
	label, ok := strings.CutSuffix(string(host), "."+string(suffix))
	return ok && label != ""
}

// schemeAllowed reports whether scheme may reach host: https is permitted for
// any host, plain http only for a loopback target (a local service — reaching
// localhost/LAN is a primary import use case, ADR 0006 §8). Every other
// combination (http to a remote host, or a non-http(s) scheme) is rejected.
func schemeAllowed(scheme urlScheme, host Host) bool {
	switch scheme {
	case "https":
		return true
	case "http":
		return IsLoopback(host)
	default:
		return false
	}
}

// IsLoopback reports whether host targets the local machine: the name
// "localhost" (case-insensitive) or any loopback IP literal (127.0.0.0/8, ::1).
// It is the shared classifier the importer's scheme policy and serve's
// import-exposure guard both consult.
func IsLoopback(host Host) bool {
	if strings.EqualFold(string(host), "localhost") {
		return true
	}
	ip := net.ParseIP(string(host))
	return ip != nil && ip.IsLoopback()
}
