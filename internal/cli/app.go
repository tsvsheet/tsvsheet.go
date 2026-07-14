package cli

import (
	"context"
	"log/slog"

	golog "github.com/gomatic/go-log"
	"github.com/urfave/cli/v3"
)

const (
	name        = "tsvsheet"
	usage       = "A spreadsheet for plain text: compute .tsvt templates over .tsv data."
	description = `tsvsheet computes a .tsvt template (headers, formulas, sheet operations)
against a .tsv value grid and emits the computed sheet — a two-file worksheet
kept diffable as text.

Commands:
  render    Compute a worksheet and write TSV to stdout
  parse     Emit a template's structure as JSON
  check     Validate a template (exit 0 clean / 1 diagnostics / 2 syntax)
  explain   Trace how one computed cell was produced
  serve     Serve a browser spreadsheet editor for a worksheet
  tui       Edit a worksheet in a terminal UI

Every non-interactive command reads from stdin and writes to stdout, so it
composes in unix pipelines:
  cat sheet.tsvt | tsvsheet render --data sheet.tsv | column -t`
)

// exit codes.
const (
	exitOK          = 0
	exitError       = 1
	exitSyntaxError = 2
)

// Version is the build version, set by main (ldflags -X).
var Version = "dev"

// loggerConfig holds the global logging flags, bound on the root command.
var loggerConfig golog.LoggerConfig

// Command builds the root tsvsheet command with the given version. A Before
// hook configures the default structured logger from the global flags so that
// diagnostics (and the top-level error) log consistently to stderr.
func Command(v string) *cli.Command {
	return &cli.Command{
		Name:                  name,
		Usage:                 usage,
		Description:           description,
		Version:               v,
		EnableShellCompletion: true,
		Before:                configureLogger,
		Flags:                 loggerFlags(),
		Commands: []*cli.Command{
			renderCommand(),
			parseCommand(),
			checkCommand(),
			explainCommand(),
			serveCommand(),
			tuiCommand(),
		},
	}
}

// configureLogger installs the default structured logger from the parsed
// logging flags.
func configureLogger(ctx context.Context, _ *cli.Command) (context.Context, error) {
	slog.SetDefault(loggerConfig.NewLogger(stderr))
	return ctx, nil
}

// loggerFlags builds the global --log-level / --log-format flags.
func loggerFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "log-level",
			Sources:     cli.EnvVars("TSVSHEET_LOG_LEVEL"),
			Value:       "info",
			Usage:       "Logging level (debug, info, warn, error)",
			Destination: (*string)(&loggerConfig.LogLevel),
		},
		&cli.StringFlag{
			Name:        "log-format",
			Sources:     cli.EnvVars("TSVSHEET_LOG_FORMAT"),
			Value:       "text",
			Usage:       "Log output format (text, json)",
			Destination: (*string)(&loggerConfig.LogFormat),
		},
	}
}

// Run builds and runs the CLI, returning the process exit code: 0 success,
// 2 syntax error, 1 any other error.
func Run(ctx context.Context, args []string) int {
	err := Command(Version).Run(ctx, args)
	return exitCode(err)
}

// exitCode maps a run error to a process exit code. A syntax error is exit 2,
// diagnostics are exit 1 (already printed by check, so not re-logged), and any
// other error is exit 1 and logged.
func exitCode(err error) int {
	switch {
	case err == nil:
		return exitOK
	case isSyntaxError(err):
		slog.Error("tsvsheet", "error", err)
		return exitSyntaxError
	case isDiagnostics(err):
		return exitError
	default:
		slog.Error("tsvsheet", "error", err)
		return exitError
	}
}
