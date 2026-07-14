package serve

import (
	"time"

	app "github.com/gomatic/go-app"
	"github.com/urfave/cli/v3"

	domain "github.com/gomatic/template.cli/internal/domain/serve"
)

const (
	name        = `serve`
	usage       = `Start an HTTP server.`
	description = `Start an HTTP server that listens for incoming requests.

This command demonstrates:
  - Long-running processes
  - Graceful shutdown via context cancellation
  - Signal handling (Ctrl-C/SIGTERM)
  - Server configuration via flags and environment variables
  - Structured logging for server lifecycle events

The server will run until interrupted by a signal (Ctrl-C) or context cancellation.
On shutdown, it will gracefully finish processing in-flight requests within the
shutdown timeout period.

Examples:
  template.cli serve
  template.cli serve --host 0.0.0.0 --port 8080
  template.cli serve --shutdown-timeout 30s
  HOST=0.0.0.0 PORT=3000 template.cli serve

Endpoints:
  GET /health - Health check endpoint (returns {"status":"ok"})
  GET /       - Root endpoint (returns plain text)`
)

const (
	hostFlag            = "host"
	portFlag            = "port"
	shutdownTimeoutFlag = "shutdown-timeout"
)

var (
	cfg       domain.Config
	runAction = domain.Run
)

// Command returns the CLI command definition.
func Command() *cli.Command {
	return &cli.Command{
		Name:        name,
		Usage:       usage,
		Description: description,
		Action:      app.Default(&cfg, runAction),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        hostFlag,
				Aliases:     []string{"H"},
				Sources:     cli.EnvVars("HOST"),
				Value:       "127.0.0.1",
				Usage:       "Host address to bind to (use 0.0.0.0 for all interfaces)",
				Destination: (*string)(&cfg.Host),
			},
			&cli.IntFlag{
				Name:        portFlag,
				Aliases:     []string{"p"},
				Sources:     cli.EnvVars("PORT"),
				Value:       8080,
				Usage:       "Port number to listen on",
				Destination: (*int)(&cfg.Port),
			},
			&cli.DurationFlag{
				Name:        shutdownTimeoutFlag,
				Aliases:     []string{"t"},
				Sources:     cli.EnvVars("SHUTDOWN_TIMEOUT"),
				Value:       10 * time.Second,
				Usage:       "Maximum time to wait for graceful shutdown",
				Destination: &cfg.ShutdownTimeout,
			},
		},
	}
}
