package cli

import (
	"context"
	"log/slog"
	"net/url"

	httpserver "github.com/gomatic/go-httpserver"
	"github.com/urfave/cli/v3"

	"github.com/tsvsheet/tsvsheet.go/internal/dataserve"
	"github.com/tsvsheet/tsvsheet.go/internal/importer"
)

// The data-base flag, registered on every command that computes. It names where
// a sheet's relative IMPORT* references resolve — so the sheet itself carries no
// scheme, host, or port and stays portable across machines.
const (
	flagData = "data"

	usageData = "Data base for relative IMPORT* references: a URL naming an existing " +
		"data server, or a directory to publish for this run (bound to loopback, stopped on exit)"
)

// dataSpec is the raw --data value before it is classified: either a URL naming
// an existing data server, or a directory to publish for this run.
type dataSpec string

// dataCloser releases a data server started for the duration of a command. It is
// a no-op when --data named an existing server, or was not given at all, so every
// caller can defer it unconditionally.
type dataCloser func() error

// noCloseData is the release for a run that started no server.
func noCloseData() error { return nil }

// dataFlags returns the --data flag, appended beside importFlags() on every
// command that computes a sheet.
func dataFlags() []cli.Flag {
	return []cli.Flag{&cli.StringFlag{
		Name:    flagData,
		Sources: cli.EnvVars("TSV_DATA"),
		Value:   "", // empty means no data base; a scheme-carrying value names a server
		Usage:   usageData,
	}}
}

// resolveData builds the data base from --data. A value carrying a scheme names
// an existing server and is validated by the same transport policy every import
// obeys — https anywhere, plain http only to loopback, with no exemption for
// being named on the command line — and a bad one fails here at startup, never
// as a deferred #IMPORT! at compute time. Anything else is a directory: a server
// is started on loopback for this run, and the returned closer stops it.
func resolveData(c *cli.Command) (importer.DataBase, dataCloser, error) {
	spec := dataSpec(c.String(flagData))
	if spec == "" {
		return importer.DataBase{}, noCloseData, nil
	}
	if !spec.hasScheme() {
		return startScopedData(dataserve.Root(spec))
	}
	base, err := importer.NewDataBase(importer.BaseURL(spec))
	return base, noCloseData, err
}

// hasScheme reports whether raw is an absolute URL, which is what distinguishes
// "use this server" from "publish this directory". A single-letter scheme is
// treated as a path so a Windows drive ("C:\data") stays a directory.
func (d dataSpec) hasScheme() bool {
	parsed, err := url.Parse(string(d))
	return err == nil && len(parsed.Scheme) > 1
}

// startScopedData publishes dir on loopback for the life of one command and
// returns its base plus the closer that stops it.
func startScopedData(dir dataserve.Root) (importer.DataBase, dataCloser, error) {
	server, err := dataserve.Start(dir, dataserve.LoopbackAny)
	if err != nil {
		return importer.DataBase{}, noCloseData, err
	}
	return importer.LoopbackBase(importer.HostPort(server.Addr())), server.Close, nil
}

// dataConfig binds the data command's published directory and bind address.
type dataConfig struct {
	root dataserve.Root
	host string
	port int
}

// dataCommand builds the `data` command: the same server --data starts, run in
// the foreground until interrupted.
func dataCommand() *cli.Command {
	cfg := dataConfig{}
	return &cli.Command{
		Name:      cmdData,
		Usage:     "Serve a directory of tabular data for relative import references.",
		ArgsUsage: "<dir>",
		Description: `Publish a directory of .tsv and .csv files over HTTP so sheets computed
elsewhere resolve their relative IMPORT* references against it — pass the
printed base URL as another command's --data.

Only .tsv and .csv are served: read-only, GET only, confined to the directory,
with no listing. It binds 127.0.0.1 by default, and while it runs the directory
is readable by every process on this machine; do not bind a non-loopback --host
on an untrusted network.

For a single command prefer the flag — ` + "`tsv render --data ./data sheet.tsvt`" + ` —
which starts this same server and stops it on exit.

Examples:
  tsv data ./data
  tsv data --port 8137 ./data`,
		Flags: []cli.Flag{
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
				Value:       0, // the documented default: 0 asks the kernel for a free port
				Usage:       "Port to listen on (0 asks the kernel for a free one)",
				Destination: &cfg.port,
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			cfg.root = dataserve.Root(positional(c.Args().Slice()).text(0))
			if cfg.root == "" {
				return missingArgument(c)
			}
			return runData(ctx, cfg)
		},
	}
}

// runData publishes cfg.root over HTTP until ctx is cancelled (Ctrl-C). The
// directory is required — there is no stdin equivalent for a directory, and
// defaulting to the working directory would publish whatever happened to be
// there — and the command shows its help rather than calling here when it is
// omitted.
func runData(ctx context.Context, cfg dataConfig) error {
	handler, err := dataserve.Handler(cfg.root)
	if err != nil {
		return err
	}
	warnNonLoopback(bindHost(cfg.host), loopbackBind(importer.IsLoopback(importer.Host(cfg.host))))
	server := httpserver.New(
		slog.Default(),
		httpserver.Host(cfg.host),
		httpserver.Port(cfg.port),
		observed(handler, "tsv_data"),
	)
	slog.Info("serving data", "url", "http://"+server.Addr()+"/", "root", string(cfg.root))
	return server.Serve(ctx, shutdownTimeout)
}
