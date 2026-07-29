// Package importer's data-base half: the operator-named base that a relative
// IMPORT* reference resolves against, and the confinement that keeps a
// reference under it.
package importer

import (
	"net/url"
	"path"
	"strings"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

// BaseURL is an operator-supplied data base, before validation — the --data
// value when it names an existing server.
type BaseURL string

// HostPort is the address a data server this process started is listening on.
type HostPort string

// basePath is a data base's URL path, normalized to a trailing-slashed prefix
// that a resolved reference must stay under.
type basePath string

// DataBase is the operator-named base that a relative import reference resolves
// against — the `--data` value. The zero DataBase configures no base, so every
// relative reference is ErrImportNoBase.
type DataBase struct {
	url  *url.URL
	root basePath
}

// NewDataBase validates an operator-supplied base URL. Its scheme must be
// permitted for its host by the same policy every import obeys — https
// anywhere, plain http only to loopback — with no exemption for being named on
// the command line: a base fetched in cleartext would put the values a sheet
// computes from on the wire, readable and rewritable in transit. The path is
// normalized to a trailing slash so a relative reference resolves *under* the
// base rather than beside it.
func NewDataBase(raw BaseURL) (DataBase, error) {
	parsed, err := url.Parse(string(raw))
	if err != nil {
		return DataBase{}, constants.ErrImportURL.With(err)
	}
	if !schemeAllowed(urlScheme(parsed.Scheme), Host(parsed.Hostname())) {
		return DataBase{}, constants.ErrImportScheme
	}
	root := basePrefix(basePath(parsed.Path))
	parsed.Path = string(root)
	return DataBase{url: parsed, root: root}, nil
}

// basePrefix normalizes a base path to a cleaned, trailing-slashed prefix, so
// containment cannot be satisfied by a sibling whose name merely starts the
// same way ("/teamster/x" is not under "/team/").
func basePrefix(p basePath) basePath {
	cleaned := basePath(path.Clean("/" + string(p)))
	if cleaned == "/" {
		return "/"
	}
	return cleaned + "/"
}

// LoopbackBase builds a base for a data server this process just started at
// hostport. The URL is constructed rather than parsed: a base we built
// ourselves cannot be malformed, and an error branch nobody can reach is worse
// than no branch at all.
func LoopbackBase(hostport HostPort) DataBase {
	return DataBase{url: &url.URL{Scheme: "http", Host: string(hostport), Path: "/"}, root: "/"}
}

// Configured reports whether a base was supplied. A zero DataBase refuses
// every relative reference.
func (b DataBase) Configured() bool { return b.url != nil }

// resolve resolves a relative reference against the base and confines it to it:
// the resolved path must remain under the base's prefix, so no depth of ".."
// can climb out.
func (b DataBase) resolve(ref *url.URL) (*url.URL, error) {
	if !b.Configured() {
		return nil, constants.ErrImportNoBase
	}
	target := b.url.ResolveReference(ref)
	if !strings.HasPrefix(path.Clean(target.Path)+"/", string(b.root)) {
		return nil, constants.ErrImportEscape
	}
	return target, nil
}

// baseOrigin reports whether a reference was resolved against the data base. A
// base reference is authorized by the operator having named that base, so the
// host allowlist — which governs URLs written inside a sheet — does not apply
// to it.
type baseOrigin bool
