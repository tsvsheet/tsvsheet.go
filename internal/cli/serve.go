package cli

import (
	"context"
	"fmt"

	"github.com/tsvsheet/go-tsvsheet"
	"github.com/urfave/cli/v3"

	"github.com/tsvsheet/tsvsheet.go/internal/importer"
)

// serveConfig binds the serve command's spreadsheet path, bind address,
// path-access mode, auto-refresh cadence (a duration or an isnow pattern), the
// resource limits the editing session enforces, and the content-typed import
// fetcher (nil when imports are off) with its refresh cache.
type serveConfig struct {
	fetcher      tsvsheet.Fetcher
	cache        *importer.Cache
	source       sourcePath
	host         string
	refresh      string
	limits       tsvsheet.Limits
	port         int
	isUnconfined pathAccess
}

// defaultServeHost is the loopback address serve binds by default — a single-user
// local editor stays off the network unless the operator opts in.
const defaultServeHost = "127.0.0.1"

// flagRefreshInterval sets the browser's auto-refresh cadence for volatile
// (clock-dependent) cells.
const flagRefreshInterval = "refresh-interval"

// serveCommand builds the `serve` command.
// serveCommand builds the `serve` command group: one front door for every
// surface this binary can serve. The sheet editor and the document API are
// separate subcommands rather than one flag-switched server, because they
// answer different questions — "let me edit this file" and "let a client edit
// documents under this directory" — and a reader should not have to infer
// which from the arguments.
func serveCommand() *cli.Command {
	return &cli.Command{
		Name:  cmdServe,
		Usage: "Serve a surface: a browser editor for one sheet, or the document API for a directory.",
		Description: `Every server this binary can run, under one command.

  tsv serve sheet <sheet.tsvt>   a browser editor for one spreadsheet file
  tsv serve api --root DIR       the document API: read, edit by op batches,
                                 watch, and read computed values over HTTP

Both bind loopback by default. The editor writes host files; the API confines
every path to its root.`,
		Commands: []*cli.Command{sheetCommand(), apiCommand()},
		// A bare `tsv serve` or an old-style `tsv serve sheet.tsvt` lands
		// here. Without an Action, urfave answers the latter with "No help
		// topic for 'sheet.tsvt'" and exits 3 — a code this repo does not use,
		// produced by an os.Exit inside the parser that bypasses the exit-code
		// mapping entirely. Since `serve` took a sheet positional until this
		// change, that is precisely the path every existing invocation takes,
		// so it says what happened and exits like any other usage mistake.
		Action: func(_ context.Context, c *cli.Command) error {
			if named := c.Args().First(); named != "" {
				_, _ = fmt.Fprintf(stderr, serveMovedNotice, named, named)
			}
			return missingArgument(c)
		},
	}
}

// startServe is the serve step, indirected so a test can assert what the
// command decided — the flags a server is built from are as much a contract as
// the server's behaviour, and a mis-wired flag is invisible from the outside.
var startServe = runServe

// serveMovedNotice explains the one breaking change of this command group to
// whoever typed the form that used to work.
const serveMovedNotice = "tsv serve now takes a surface: did you mean `tsv serve sheet %s`?\n" +
	"  tsv serve sheet %s   browser editor for one spreadsheet\n" +
	"  tsv serve api --root DIR   the document API for a directory\n"

// sheetCommand builds the `serve sheet` command: the single-document browser
// editor. It is an explicit subcommand because `serve` also takes a positional
// sheet: a file literally named "sheet" would otherwise be ambiguous to a
// reader and to the parser.
func sheetCommand() *cli.Command {
	isUnconfined := false
	cfg := serveConfig{}
	return &cli.Command{
		Name:      cmdServeSheet,
		Usage:     "Serve a browser spreadsheet editor for one sheet.",
		ArgsUsage: argsUsageSheet,
		Description: `Host a local web spreadsheet backed by the tsvsheet engine: edit any cell
(a value or an =formula) in the browser, recompute live, and save. The sheet
is a required positional file path (serve saves edits back to it, so stdin is
not accepted).

This is a single-user local editor: the browser reads and WRITES host files
(Save overwrites the sheet, and references can read its directory). It binds
127.0.0.1 by default and refuses cross-origin requests; do not bind a
non-loopback --host on an untrusted network.

Examples:
  tsv serve sheet sheet.tsvt
  tsv serve sheet --host 0.0.0.0 --port 8080 sheet.tsvt`,
		Flags: append([]cli.Flag{
			&cli.StringFlag{
				Name:        "host",
				Sources:     cli.EnvVars("HOST"),
				Value:       defaultServeHost,
				Usage:       "Host address to bind",
				Destination: &cfg.host,
			},
			&cli.IntFlag{
				Name:        "port",
				Aliases:     []string{"p"},
				Sources:     cli.EnvVars("PORT"),
				Value:       8080,
				Usage:       "Port to listen on",
				Destination: &cfg.port,
			},
			&cli.BoolFlag{
				Name:        flagAllowAnyPaths,
				Sources:     cli.EnvVars("TSV_ALLOW_ANY_PATHS"),
				Usage:       usageAllowAnyPaths,
				Destination: &isUnconfined,
			},
			&cli.StringFlag{
				Name:        flagRefreshInterval,
				Sources:     cli.EnvVars("TSV_REFRESH_INTERVAL"),
				Value:       "", // empty selects the documented auto default: 1s with clock functions, else off
				Usage:       `Auto-recompute the browser view: a duration (30s) or an isnow pattern ("M-F +[30mn] >=9 <=17"); 0 disables. Default: 1s when the sheet has clock functions (TODAY/NOW/ISNOW), else off`,
				Destination: &cfg.refresh,
			},
		}, append(importFlags(), dataFlags()...)...),
		Action: func(ctx context.Context, c *cli.Command) error {
			base, closeData, err := resolveData(c)
			if err != nil {
				return err
			}
			defer func() { _ = closeData() }()
			fetcher, cache, err := resolveImport(c, base)
			if err != nil {
				return err
			}
			cfg.source = positional(c.Args().Slice()).at(0)
			if cfg.source.isStdin() {
				return missingArgument(c)
			}
			cfg.isUnconfined = pathAccess(isUnconfined)
			cfg.limits = maxCellsLimits(c)
			cfg.fetcher, cfg.cache = fetcher, cache
			return startServe(ctx, cfg)
		},
	}
}
