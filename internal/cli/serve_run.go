package cli

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	httpserver "github.com/gomatic/go-httpserver"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
	"github.com/tsvsheet/tsvsheet.go/internal/importer"
	"github.com/tsvsheet/tsvsheet.go/internal/refresh"
	"github.com/tsvsheet/tsvsheet.go/internal/serve"
	"github.com/tsvsheet/tsvsheet.go/internal/session"
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
// overrides everything when given; otherwise the sheet's own volatile(…)
// cadences are unioned to the soonest next instant, with a 1s default for any
// volatile() carrying no schedule of its own, else off. A malformed pattern is
// an error.
func buildRefresh(spec refresh.Spec, sess *session.Session) (refresh.Next, error) {
	if spec != "" {
		next, err := refresh.Parse(spec)
		if err != nil {
			return nil, tsvsheet.ErrInvalidValue.With(err, flagRefreshInterval, string(spec))
		}
		return next, nil
	}
	return refresh.Union(scheduleSpecs(sess.VolatileSchedules()), refresh.Every(defaultRefresh))
}

// scheduleSpecs adapts a session's volatile-cadence strings to refresh specs.
func scheduleSpecs(schedules []string) []refresh.Spec {
	specs := make([]refresh.Spec, len(schedules))
	for i, s := range schedules {
		specs[i] = refresh.Spec(s)
	}
	return specs
}

// saver builds the persist function: it writes the session's current source
// back to the spreadsheet file. The plain func() error is assignable to both
// serve.Saver and tui.Saver.
//
// The write is confined and atomic. Confined: it goes through os.Root on the
// sheet's own directory, so a sheet path that is a symlink out of that directory
// cannot be used to overwrite the file it points at. Atomic: the bytes land in a
// temporary file that is then renamed over the sheet, so an interrupted save
// leaves the previous sheet intact instead of a truncated one — a plain
// os.WriteFile truncates first and would destroy the sheet if the process died
// mid-write.
func saver(sess *session.Session, source sourcePath) func() error {
	dir, file := filepath.Split(filepath.Clean(string(source)))
	if dir == "" {
		dir = "."
	}
	return func() error { return saveAtomic(dir, file, sess.Source()) }
}

// saveAtomic writes data as file inside dir, via a temporary file in that same
// directory and a rename. Same-directory is required: a rename is only atomic
// within one filesystem.
func saveAtomic(dir, file string, data []byte) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return tsvsheet.ErrWriteFile.With(err, dir)
	}
	defer func() { _ = root.Close() }()
	tmp := file + tempSuffix
	if err := writeFileIn(root, tmp, data, filePerm); err != nil {
		return tsvsheet.ErrWriteFile.With(err, tmp)
	}
	if err := renameIn(root, tmp, file); err != nil {
		_ = root.Remove(tmp)
		return tsvsheet.ErrWriteFile.With(err, file)
	}
	return nil
}

// tempSuffix names the staging file a save renames from.
const tempSuffix = ".tsv-save"

// The two filesystem operations a save performs, as package vars so a test can
// force each failure. Go offers no way to make a real write or rename fail at a
// direct call site, and each branch carries a contract worth asserting: whichever
// step fails, the previous sheet must still be on disk and no staging file may be
// left behind. Tests stub these and must restore them, so they cannot run in
// parallel with each other.
var (
	writeFileIn = func(root *os.Root, name string, data []byte, perm os.FileMode) error {
		return root.WriteFile(name, data, perm)
	}
	renameIn = func(root *os.Root, from, to string) error { return root.Rename(from, to) }
)
