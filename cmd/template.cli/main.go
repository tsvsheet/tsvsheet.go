package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sort"

	app "github.com/gomatic/go-app"
	"github.com/gomatic/go-log"
	"github.com/urfave/cli/v3"

	"github.com/gomatic/template.cli/internal/app/commands/config"
	"github.com/gomatic/template.cli/internal/app/commands/greet"
	"github.com/gomatic/template.cli/internal/app/commands/process"
	"github.com/gomatic/template.cli/internal/app/commands/rename"
	"github.com/gomatic/template.cli/internal/app/commands/serve"
)

const (
	argUsage    = ``
	description = `A comprehensive example CLI that demonstrates:
  - Command hierarchy with parent and child commands
  - Various flag types (string, bool, int)
  - Positional arguments
  - Reading from stdin and files
  - Writing to stdout
  - Unix pipe composition
  - Environment variable support
  - Structured logging
  - Error handling patterns

This CLI follows the standards defined in the CLI Implementation Rules document.

Available Commands:
  config         - Manage configuration (demonstrates hierarchical commands with get/set/list)
  greet          - Generate greeting messages (demonstrates flags and positional args)
  process        - Process text line by line (demonstrates stdin/stdout)
  rename         - Rename this template's identity to a new project (cloned-from-template scaffolding)
  serve          - Start HTTP server (demonstrates long-running processes and graceful shutdown)

Command Naming Convention:
  Commands follow a "noun... verb" pattern (resource-oriented design).
  Example: "config get", "config set", "config list"
  This makes commands discoverable and follows git/docker/gh patterns.
  Never use hyphens in command names - use hierarchy levels instead.

Version:
  Use --version flag (built-in urfave/cli support)`
	envName   = "TEMPLATE_CLI"
	envPrefix = envName + "_"
	name      = `template.cli`
	usage     = `Example CLI demonstrating best practices.`
)

var (
	appCreator    = createApp
	loggerConfig  log.LoggerConfig
	loggerCreator = productionLogger
)

// productionLogger builds the application logger from the parsed logging flags.
// It is invoked from the root Before hook, after flag parsing has populated
// loggerConfig, so --log-level and --log-format take effect.
func productionLogger(_ *cli.Command) *slog.Logger {
	return loggerConfig.NewLogger(os.Stderr)
}

// version is the application version.
// Set via ldflags: -X main.version=1.0.0
var version = "dev"

// osExit is indirected so tests can observe the process exit code.
var osExit = os.Exit

func main() { osExit(run(os.Args)) }

// run builds and executes the CLI, returning the process exit code. Keeping the
// exit code as a return value (rather than calling os.Exit here) makes the whole
// run path testable.
func run(args []string) int {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	if err := appCreator(loggerCreator).Run(ctx, args); err != nil {
		slog.Error("Application error", "error", err)
		return 1
	}
	return 0
}

// createApp constructs the definition of the CLI.
func createApp(getLogger app.GetLoggerFunc) *cli.Command {
	cliApp := &cli.Command{
		Name:                  name,
		Usage:                 usage,
		ArgsUsage:             argUsage,
		Description:           description,
		Version:               version,
		EnableShellCompletion: true,
		Commands: []*cli.Command{
			config.Command(),
			greet.Command(),
			process.Command(),
			rename.Command(),
			serve.Command(),
		},
		Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
			c.Root().Metadata[app.LoggerMetadataKey] = getLogger(c)
			return ctx, nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "log-level",
				Sources:     cli.EnvVars(envPrefix + "LOG_LEVEL"),
				Value:       "info",
				Usage:       "Set the logging level (debug, info, warn, error)",
				Destination: (*string)(&loggerConfig.LogLevel),
			},
			&cli.StringFlag{
				Name:        "log-format",
				Sources:     cli.EnvVars(envPrefix + "LOG_FORMAT"),
				Value:       "text",
				Usage:       "Set the log output format (text, json)",
				Destination: (*string)(&loggerConfig.LogFormat),
			},
		},
	}

	sort.Sort(cli.FlagsByName(cliApp.Flags))

	return cliApp
}
