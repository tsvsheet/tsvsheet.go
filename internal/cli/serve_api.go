package cli

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/tsvsheet/tsvsheet.api/api"
	"github.com/tsvsheet/tsvsheet.api/store"
	"github.com/urfave/cli/v3"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

// The `serve api` flag names.
const (
	flagRoot      = "root"
	flagNoCompute = "no-compute"
)

// defaultAPIAddress is where `serve api` listens: loopback, because the server
// carries no TLS and no authentication of its own.
const defaultAPIAddress = "127.0.0.1:8787"

// apiHeaderTimeout bounds how long a client may dawdle over its request head.
const apiHeaderTimeout = 10 * time.Second

// apiCommand builds the `serve api` command: the sheet API over a directory of
// documents.
//
// It composes rather than reimplements: the handler, the confined store, and
// the wire contract all come from github.com/tsvsheet/tsvsheet.api, so this
// binary and the standalone tsvsheet-api server stay in step. What lives here
// is the composition — which directory, which address, which limits.
func apiCommand() *cli.Command {
	return &cli.Command{
		Name:  cmdServeAPI,
		Usage: "Serve the document API for a directory of spreadsheets.",
		Description: `Serve the documents under --root over HTTP: read a sheet, edit it by sending
an edits batch (a TSV stream of semantic operations), watch it change, and read
its computed values in the media types the language's IMPORT* functions request
— so a served sheet can be imported by another sheet.

Every path is confined to the root, every mutation is conditioned on the
revision it expects, and the address must be loopback: this server carries no
TLS and no authentication of its own, so a wider exposure is refused rather
than served insecurely.

Examples:
  tsv serve api --root ./sheets
  tsv serve api --root ./sheets --addr 127.0.0.1:9000 --no-compute`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    flagRoot,
				Sources: cli.EnvVars("TSV_ROOT"),
				// Deliberately no directory default: a missing --root is a
				// usage mistake (help, exit 2), never an implicit cwd.
				Value: "",
				Usage: "directory of .tsvt documents to serve",
			},
			&cli.StringFlag{
				Name:    "addr",
				Sources: cli.EnvVars("TSV_ADDR"),
				Usage:   "loopback address to bind",
				Value:   defaultAPIAddress,
			},
			&cli.BoolFlag{
				Name:    flagNoCompute,
				Sources: cli.EnvVars("TSV_NO_COMPUTE"),
				Usage:   "serve the document plane only: no computed reads, no computed events",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			// A missing --root is a usage mistake, reported the way every other
			// command reports one: its own help, exit 2. urfave's Required
			// would exit 1 with a bare message, which is the code this repo
			// reserves for a runtime failure.
			if c.String(flagRoot) == "" {
				return missingArgument(c)
			}
			server, err := buildAPIServer(c)
			if err != nil {
				return err
			}
			return listenAndServe(ctx, server)
		},
	}
}

// buildAPIServer assembles the HTTP server from the parsed flags: a store
// confined to the root, the API handler over it, and a verified bind address.
func buildAPIServer(c *cli.Command) (*http.Server, error) {
	address := serveAddress(c.String("addr"))
	if err := requireLoopback(address); err != nil {
		return nil, err
	}
	limits := maxCellsLimits(c)
	documents, err := store.Open(store.RootDir(c.String(flagRoot)), limits)
	if err != nil {
		return nil, err
	}
	handler := api.NewHandler(api.Config{
		Port:           documents,
		Limits:         limits,
		ComputeEnabled: api.ComputePlane(!c.Bool(flagNoCompute)),
		Clock:          time.Now,
	})
	return &http.Server{
		Addr:              string(address),
		Handler:           handler,
		ReadHeaderTimeout: apiHeaderTimeout,
	}, nil
}

// serveAddress is a host:port a server binds.
type serveAddress string

// requireLoopback refuses any bind address that is not loopback. The API
// carries no TLS and no authentication, so exposure is an error rather than a
// warning: a warning is read once and a bind lasts until the process ends.
func requireLoopback(address serveAddress) error {
	host, _, err := net.SplitHostPort(string(address))
	if err != nil {
		return constants.ErrServeAddress.With(err, "addr", string(address))
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return constants.ErrServeExposed.With(nil, "addr", string(address))
	}
	return nil
}

// apiShutdownTimeout bounds how long a shutdown waits for in-flight requests.
const apiShutdownTimeout = 5 * time.Second

// listenAndServe performs the blocking serve, indirected so a test exercises
// the whole build path without binding a socket.
//
// It shuts down on the context rather than dying with it: an interrupted
// server that drops a request mid-write leaves the client unable to tell a
// refused edit from a lost one, which is the distinction the whole
// conditional-write discipline exists to preserve.
var listenAndServe = func(ctx context.Context, server *http.Server) error {
	serving := make(chan error, 1)
	go func() { serving <- server.ListenAndServe() }()
	select {
	case err := <-serving:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), apiShutdownTimeout)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}
