package cli

import (
	"context"
	"log/slog"

	golog "github.com/gomatic/go-log"
	"github.com/tsvsheet/go-tsvsheet"
	"github.com/urfave/cli/v3"
)

const (
	name        = "tsv"
	usage       = "A spreadsheet in plain text: a .tsvt grid of values and =formulas."
	description = `tsv computes a .tsvt spreadsheet — a TAB-separated grid whose cells are
literal values or =formulas that address other cells in A1 notation (B2,
D2:D4) — and emits the computed grid, kept diffable as text.

The sheet is a positional argument; an omitted sheet (or "-") is read from
stdin.

Non-interactive commands write to stdout, so they compose in unix pipelines:
  tsv render sheet.tsvt | column -t
  cat sheet.tsvt | tsv check`
)

// exit codes.
const (
	exitOK          = 0
	exitError       = 1
	exitSyntaxError = 2
)

// command names.
const (
	cmdRender     = "render"
	cmdParse      = "parse"
	cmdFromJSON   = "from-json"
	cmdCheck      = "check"
	cmdExplain    = "explain"
	cmdEval       = "eval"
	cmdApply      = "apply"
	cmdData       = "data"
	cmdServe      = "serve"
	cmdServeSheet = "sheet"
	cmdServeAPI   = "api"
	cmdTUI        = "tui"
	cmdComplete   = "completion"
	cmdMan        = "man"
)

// builtinCompletionName renames urfave/cli's auto-added (hidden) shell-completion
// command so it does not collide with this repo's own visible `completion`
// command. EnableShellCompletion still drives on-the-fly <TAB> completion via the
// --generate-shell-completion flag; the renamed built-in only supplies the
// per-shell script templates that the `completion` command delegates to.
const builtinCompletionName = "__completion"

// argSheetOptional is the ArgsUsage for commands whose sheet argument may be
// omitted to read stdin.
const argSheetOptional = "[sheet]"

// flagMaxCells names the global resource-cap flag.
const flagMaxCells = "max-cells"

// Version is a build version string, supplied by main (ldflags -X) and threaded
// into the command rather than held in a package-level variable.
type Version string

// The global logging flag names, read back from the parsed command.
const (
	flagLogLevel  = "log-level"
	flagLogFormat = "log-format"
)

// Command builds the root tsv command with the given version. A Before
// hook configures the default structured logger from the global flags so that
// diagnostics (and the top-level error) log consistently to stderr.
func Command(v Version) *cli.Command {
	return &cli.Command{
		Name:                       name,
		Usage:                      usage,
		Description:                description,
		Version:                    string(v),
		EnableShellCompletion:      true,
		ShellCompletionCommandName: builtinCompletionName,
		DefaultCommand:             cmdRender,
		Before:                     configureLogger,
		Flags:                      append(loggerFlags(), maxCellsFlag()),
		Commands: []*cli.Command{
			renderCommand(),
			parseCommand(),
			fromJSONCommand(),
			checkCommand(),
			explainCommand(),
			evalCommand(),
			applyCommand(),
			serveCommand(),
			dataCommand(),
			tuiCommand(),
			completionCommand(),
			manCommand(),
		},
	}
}

// configureLogger installs the default structured logger from the parsed
// logging flags. The --max-cells resource cap is applied per command (threaded
// through the compute path and the editing session), not via a global here.
func configureLogger(ctx context.Context, c *cli.Command) (context.Context, error) {
	cfg := golog.LoggerConfig{
		Level:  golog.Level(c.String(flagLogLevel)),
		Format: golog.Format(c.String(flagLogFormat)),
	}
	slog.SetDefault(cfg.NewLogger(stderr))
	return ctx, nil
}

// maxCellsFlag caps how large any single formula result or grid may grow, so an
// untrusted sheet cannot exhaust memory. Zero (the default) keeps DefaultLimits.
func maxCellsFlag() cli.Flag {
	return &cli.IntFlag{
		Name:    flagMaxCells,
		Sources: cli.EnvVars("TSV_MAX_CELLS"),
		Usage:   "cap on the cells, grid dimension, and bytes a single formula result or grid may reach (0 = built-in default)",
		Value:   0,
	}
}

// maxCellsLimits resolves the global --max-cells flag to resource limits: a
// positive cap is the single ceiling for every budget — the cells one written
// reference may span (SpanCells), the cells and bytes one formula result may
// reach, and the grid dimension an edit may grow to; zero (the default) keeps
// the engine's generous DefaultLimits. An over-budget reference or result
// computes to #LIMIT! (SPECIFICATION §6).
func maxCellsLimits(c *cli.Command) tsvsheet.Limits {
	if n := c.Root().Int(flagMaxCells); n > 0 {
		return tsvsheet.Limits{ResultCells: n, GridDim: n, ResultBytes: n, SpanCells: n}
	}
	return tsvsheet.DefaultLimits()
}

// loggerFlags builds the global --log-level / --log-format flags.
//
// They deliberately carry no Destination. A Destination writes through a pointer
// fixed when the flag is built, so a package-level target is shared by every
// root command in the process — two commands parsed concurrently write the same
// word, which is a data race, and the second parse silently overwrites the
// first's configuration. The values are read back from the parsed command in
// configureLogger instead, where they belong to that command alone.
func loggerFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    flagLogLevel,
			Sources: cli.EnvVars("TSV_LOG_LEVEL"),
			Value:   "info",
			Usage:   "Logging level (debug, info, warn, error)",
		},
		&cli.StringFlag{
			Name:    flagLogFormat,
			Sources: cli.EnvVars("TSV_LOG_FORMAT"),
			Value:   "text",
			Usage:   "Log output format (text, json)",
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
// a usage mistake is exit 2 with its help already printed, diagnostics are
// exit 1 (already printed by check), and any other error is exit 1 and logged.
// Only the last two cases log: a mistake the user can see on screen does not
// belong in the log stream a real failure shares.
func exitCode(err error) int {
	switch {
	case err == nil:
		return exitOK
	case isSyntaxError(err):
		slog.Error(name, "error", err)
		return exitSyntaxError
	case isUsage(err):
		return exitSyntaxError
	case isDiagnostics(err):
		return exitError
	default:
		slog.Error(name, "error", err)
		return exitError
	}
}
