package cli

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	httpserver "github.com/gomatic/go-httpserver"
	"github.com/uplang/go-tsvsheet"

	"github.com/uplang/tsvsheet.go/internal/constants"
	"github.com/uplang/tsvsheet.go/internal/importer"
	"github.com/uplang/tsvsheet.go/internal/refresh"
	"github.com/uplang/tsvsheet.go/internal/serve"
	"github.com/uplang/tsvsheet.go/internal/session"
)

// shutdownTimeout bounds graceful shutdown when serve is interrupted.
const shutdownTimeout = 5 * time.Second

// filePerm is the mode for a saved spreadsheet file.
const filePerm = 0o600

// runServe loads the spreadsheet into a session, hosts it over HTTP, and serves
// until ctx is cancelled (Ctrl-C). The sheet must be a file — serve saves edits
// back to it, so stdin is not a valid source.
func runServe(ctx context.Context, cfg serveConfig) error {
	isLoopback := loopbackBind(importer.IsLoopback(importer.Host(cfg.host)))
	if err := guardImportExposure(cfg, isLoopback); err != nil {
		return err
	}
	warnNonLoopback(bindHost(cfg.host), isLoopback)
	server, err := loadServer(cfg)
	if err != nil {
		return err
	}
	http := httpserver.New(slog.Default(), httpserver.Host(cfg.host), httpserver.Port(cfg.port), server.Handler())
	slog.Info("serving spreadsheet", "url", "http://"+http.Addr())
	return http.Serve(ctx, shutdownTimeout)
}

// bindHost is serve's bind address; loopbackBind reports whether that address
// is a loopback (local-only) address.
type (
	bindHost     string
	loopbackBind bool
)

// guardImportExposure refuses to start when imports are enabled on a non-loopback
// bind address: a network-exposed server must never fetch content-typed imports
// on its clients' behalf (ADR 0006 §8). Loopback binds and import-less serves are
// unaffected.
func guardImportExposure(cfg serveConfig, isLoopback loopbackBind) error {
	if cfg.fetcher != nil && !isLoopback {
		return constants.ErrImportServeExposed.With(nil, "host", cfg.host)
	}
	return nil
}

// warnNonLoopback warns once that a non-loopback bind exposes the sheet's
// directory (the file-serving concern, distinct from the import guard above);
// loopback binds are silent.
func warnNonLoopback(host bindHost, isLoopback loopbackBind) {
	if isLoopback {
		return
	}
	slog.Warn(
		"serving on a non-loopback address exposes the sheet's directory to the network; the browser editor reads and writes host files",
		"host",
		string(host),
	)
}

// defaultRefresh is the auto-refresh interval applied when a sheet has volatile
// functions and no interval was named on the command line.
const defaultRefresh = time.Second

// loadServer reads the spreadsheet file into a session and builds the HTTP
// server with a saver and the effective auto-refresh cadence.
func loadServer(cfg serveConfig) (serve.Server, error) {
	sess, persist, err := loadEditable(cfg.source, cfg.isUnconfined, cfg.limits, cfg.fetcher)
	if err != nil {
		return serve.Server{}, err
	}
	wireRefresh(sess, cfg.cache)
	next, err := buildRefresh(refresh.Spec(cfg.refresh), sess)
	if err != nil {
		return serve.Server{}, err
	}
	return serve.NewServer(sess, persist, next), nil
}

// buildRefresh is the auto-refresh cadence shared by serve and the TUI: an
// explicit --refresh-interval (a duration or an isnow pattern, or 0 to disable)
// when spec is given, else a 1s default when the sheet has clock-dependent
// functions (TODAY/NOW/ISNOW), else off. A malformed pattern is an error.
func buildRefresh(spec refresh.Spec, sess *session.Session) (refresh.Next, error) {
	if spec != "" {
		next, err := refresh.Parse(spec)
		if err != nil {
			return nil, tsvsheet.ErrInvalidValue.With(err, flagRefreshInterval, string(spec))
		}
		return next, nil
	}
	if sess.IsVolatile() {
		return refresh.Every(defaultRefresh), nil
	}
	return nil, nil
}

// saver builds the persist function: it writes the session's current source
// back to the spreadsheet file. The plain func() error is assignable to both
// serve.Saver and tui.Saver.
func saver(sess *session.Session, source sourcePath) func() error {
	path := filepath.Clean(string(source))
	return func() error {
		if err := os.WriteFile(path, sess.Source(), filePerm); err != nil {
			return tsvsheet.ErrWriteFile.With(err, path)
		}
		return nil
	}
}
