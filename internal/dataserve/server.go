// Package dataserve is the local data server behind `tsv data` and the --data
// flag: it publishes a directory of tabular files to loopback so a sheet's
// relative import references resolve without the sheet ever naming a transport.
//
// It is the sole home of the extension→media-type mapping. Content-Type is
// HTTP's business, and keeping it here is what lets the engine stay
// extension-blind: nothing in the tsvsheet language ascribes meaning to a file's
// suffix, and this design must not be what changes that.
package dataserve

import (
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

// Root is the directory a server publishes. Every request is confined to it via
// os.Root — the same boundary the sheet loader applies to sheet().
type Root string

// Addr is the address a server binds. Callers pass loopback; the zero port asks
// the kernel for a free one, so a scoped run needs no port negotiation.
type Addr string

// LoopbackAny is the default bind: loopback only, kernel-assigned port.
const LoopbackAny Addr = "127.0.0.1:0"

// headerTimeout bounds how long a client may take to send its request headers,
// so a stalled peer cannot pin a connection open indefinitely.
const headerTimeout = 10 * time.Second

// mediaTypes maps a published file extension to the media type an IMPORT*
// handshake accepts. A file with any other extension is not served at all —
// this server publishes tabular data, not a directory.
var mediaTypes = map[string]string{
	".tsv": "text/tab-separated-values",
	".csv": "text/csv",
}

// Server is a running data server. Base is what a --data-configured Fetcher
// resolves relative references against.
type Server struct {
	listener net.Listener
	server   *http.Server
	base     string
}

// Start publishes root at addr and returns the running server. The returned
// Base carries the port the kernel actually assigned, so nothing downstream —
// and nothing inside a sheet — has to know it in advance.
func Start(root Root, addr Addr) (Server, error) {
	handler, err := Handler(root)
	if err != nil {
		return Server{}, err
	}
	listener, err := net.Listen("tcp", string(addr))
	if err != nil {
		return Server{}, constants.ErrDataListen.With(err)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: headerTimeout}
	go func() { _ = server.Serve(listener) }()
	return Server{
		listener: listener,
		server:   server,
		base:     "http://" + listener.Addr().String() + "/",
	}, nil
}

// Base is the URL a relative import reference resolves against.
func (s Server) Base() string { return s.base }

// Addr is the host:port the server actually bound.
func (s Server) Addr() string { return s.listener.Addr().String() }

// Close stops the server. A scoped run defers it, so the listener never
// outlives the command that started it — including on an error exit.
func (s Server) Close() error {
	if s.server == nil {
		return nil
	}
	return s.server.Close()
}

// Handler serves root read-only over GET: a request for a published extension
// is answered with that file and its media type; everything else is refused.
// There is no directory listing — a data server publishes what a sheet asks for
// by name, not an inventory of the directory it was pointed at.
func Handler(root Root) (http.Handler, error) {
	if err := checkRoot(root); err != nil {
		return nil, err
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		media, ok := mediaTypes[strings.ToLower(path.Ext(r.URL.Path))]
		if !ok {
			http.NotFound(w, r)
			return
		}
		serveFile(w, r, root, media)
	}), nil
}

// checkRoot verifies the directory is publishable now, so an operator who
// mistypes --data learns at startup instead of collecting an #IMPORT! per
// reference. The root is still opened per request: this is a startup check, not
// a cached capability.
func checkRoot(root Root) error {
	confined, err := os.OpenRoot(string(root))
	if err != nil {
		return constants.ErrDataRoot.With(err)
	}
	return confined.Close()
}

// serveFile writes the requested file. The path is cleaned and opened through
// os.Root, so a traversal cannot escape root even though the importer already
// refuses one — the two checks are independent, and this one also holds for a
// client that is not the importer.
//
// The open is non-blocking (openFlags). A blocking open of a FIFO waits for a
// writer that may never come, which would pin the handler — and, for a scoped
// server, the command that started it — indefinitely. Opening first and judging
// the mode afterwards also closes the race a stat-then-open would leave.
func serveFile(w http.ResponseWriter, r *http.Request, root Root, media string) {
	confined, err := os.OpenRoot(string(root))
	if err != nil {
		http.Error(w, "data root unavailable", http.StatusInternalServerError)
		return
	}
	defer func() { _ = confined.Close() }()
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	file, err := confined.OpenFile(name, openFlags, 0)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = file.Close() }()
	writeFile(w, file, media)
}

// writeFile streams a regular file with its media type. Anything else — a
// directory, a FIFO, a device — is refused before a single header is written.
func writeFile(w http.ResponseWriter, file *os.File, media string) {
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.Error(w, "not a regular file", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", media)
	_, _ = io.Copy(w, file)
}
