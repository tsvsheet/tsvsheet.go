package cli

import (
	"context"
	"time"

	"github.com/uplang/go-tsvsheet"
	"github.com/urfave/cli/v3"

	"github.com/uplang/tsvsheet.go/internal/importer"
	"github.com/uplang/tsvsheet.go/internal/session"
)

// The content-typed import flags (ADR 0006 §7–§8), registered on every command
// that computes. Imports are OFF by default; --allow-import enables them and
// REQUIRES at least one --import-host — enabling with an empty allowlist is a
// configuration error, not a silent deny.
const (
	flagAllowImport = "allow-import"
	flagImportHost  = "import-host"

	usageAllowImport = "Enable content-typed IMPORT* fetches (off by default); requires at least one --import-host"
	usageImportHost  = "Allowlisted import host (repeatable): an exact host or *.sub wildcard; required with --allow-import"

	importHostRequired = "specify at least one --import-host with --allow-import"
)

// The injected fetcher defaults: a per-request deadline and body-size cap sized
// for a single-user local editor.
const (
	importTimeout  = 10 * time.Second
	importMaxBytes = importer.ByteSize(1 << 20)
)

// importFlags returns the --allow-import / --import-host flags for a command
// that computes (render, parse, serve, tui). explain/check do not compute a
// session and register neither.
func importFlags() []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{Name: flagAllowImport, Usage: usageAllowImport},
		&cli.StringSliceFlag{Name: flagImportHost, Usage: usageImportHost},
	}
}

// resolveImport builds the import Fetcher and its refresh cache from the parsed
// flags: nil (imports off) when --allow-import is absent; ErrInvalidValue when
// --allow-import is set with no --import-host; otherwise a hardened net/http
// Fetcher wrapped in a cross-pass cache, returned both as the tsvsheet.Fetcher the
// engine consumes and as the concrete Cache the frontend clears on refresh.
func resolveImport(c *cli.Command) (tsvsheet.Fetcher, *importer.Cache, error) {
	if !c.Bool(flagAllowImport) {
		return nil, nil, nil
	}
	hosts := c.StringSlice(flagImportHost)
	if len(hosts) == 0 {
		return nil, nil, tsvsheet.ErrInvalidValue.With(nil, "message", importHostRequired)
	}
	cache := importer.NewCache(importer.New(importer.Config{
		AllowedHosts: hostPatterns(hosts),
		Timeout:      importTimeout,
		MaxBytes:     importMaxBytes,
	}))
	return cache, cache, nil
}

// hostPatterns converts the raw --import-host strings to allowlist patterns.
func hostPatterns(hosts []string) []importer.HostPattern {
	patterns := make([]importer.HostPattern, len(hosts))
	for i, h := range hosts {
		patterns[i] = importer.HostPattern(h)
	}
	return patterns
}

// wireRefresh registers the import cache's Clear as the session's refresh hook,
// so a frontend "refresh imports" action drops cached fetches before recomputing
// (a no-op when imports are disabled and cache is nil).
func wireRefresh(sess *session.Session, cache *importer.Cache) {
	if cache != nil {
		sess.OnRefresh(cache.Clear)
	}
}

// importedAction is streamAction that also injects the --max-cells resource
// limits and resolves the import Fetcher (nil when --allow-import is off) from
// the flags, for the one-shot compute commands (render, parse). Flag validation
// failures surface as the command's error.
func importedAction(fn func(Streams, positional, tsvsheet.Limits, tsvsheet.Fetcher) error) cli.ActionFunc {
	return func(_ context.Context, c *cli.Command) error {
		fetcher, _, err := resolveImport(c)
		if err != nil {
			return err
		}
		streams := Streams{In: stdin, Out: c.Root().Writer, Err: stderr}
		return fn(streams, positional(c.Args().Slice()), maxCellsLimits(c), fetcher)
	}
}
