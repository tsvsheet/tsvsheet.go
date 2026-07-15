package cli

import (
	"context"
	"log/slog"

	golog "github.com/gomatic/go-log"
	"github.com/urfave/cli/v3"
)

const (
	name        = "tsvsheet"
	usage       = "A spreadsheet for plain text: a .tsvt grid of values and =formulas."
	description = `tsvsheet computes a .tsvt spreadsheet — a TAB-separated grid whose cells are
literal values or =formulas that address other cells in A1 notation (B2,
D2:D4) — and emits the computed grid, kept diffable as text.

The sheet is a positional argument; an omitted sheet (or "-") is read from
stdin.

Commands:
  render  <sheet>          Compute a spreadsheet, write TSV to stdout
  parse   <sheet>          Emit a sheet's cells as JSON
  check   <sheet>          Validate (exit 0 clean / 1 diags / 2 syntax)
  explain <cell> <sheet>   Trace how one computed cell was produced
  serve   <sheet>          Browser spreadsheet editor
  tui     <sheet>          Terminal spreadsheet editor

Non-interactive commands write to stdout, so they compose in unix pipelines:
  tsvsheet render sheet.tsvt | column -t
  cat sheet.tsvt | tsvsheet check`
)

// exit codes.
const (
	exitOK          = 0
	exitError       = 1
	exitSyntaxError = 2
)

// command names.
const (
	cmdRender   = "render"
	cmdParse    = "parse"
	cmdFromJSON = "from-json"
	cmdCheck    = "check"
	cmdExplain  = "explain"
	cmdServe    = "serve"
	cmdTUI      = "tui"
)

// argSheetOptional is the ArgsUsage for commands whose sheet argument may be
// omitted to read stdin.
const argSheetOptional = "[sheet]"

// Version is a build version string, supplied by main (ldflags -X) and threaded
// into the command rather than held in a package-level variable.
type Version string

// loggerConfig holds the global logging flags, bound on the root command.
var loggerConfig golog.LoggerConfig

// Command builds the root tsvsheet command with the given version. A Before
// hook configures the default structured logger from the global flags so that
// diagnostics (and the top-level error) log consistently to stderr.
func Command(v Version) *cli.Command {
	return &cli.Command{
		Name:                  name,
		Usage:                 usage,
		Description:           description,
		Version:               string(v),
		EnableShellCompletion: true,
		DefaultCommand:        cmdRender,
		Before:                configureLogger,
		Flags:                 loggerFlags(),
		Commands: []*cli.Command{
			renderCommand(),
			parseCommand(),
			fromJSONCommand(),
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
func Run(ctx context.Context, version Version, args []string) int {
	err := Command(version).Run(ctx, args)
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
