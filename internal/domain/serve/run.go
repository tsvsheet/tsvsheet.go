package serve

import (
	"context"
	"log/slog"
	"time"

	"github.com/gomatic/go-httpserver"

	"github.com/gomatic/template.cli/internal/constants"
	"github.com/gomatic/template.cli/internal/domain"
)

const (
	minPort    = 0               // minPort is the lowest valid port (0 lets the OS choose).
	maxPort    = 65535           // maxPort is the highest valid port.
	minTimeout = 1 * time.Second // minTimeout is the smallest accepted shutdown timeout.
)

// Result is the outcome of the serve command. Serving produces no value.
type Result struct{}

// Run validates the configuration and runs the HTTP server until ctx is
// cancelled, delegating the lifecycle to the httpserver package.
func Run(ctx context.Context, logger *slog.Logger, cfg Config, _ ...domain.Argument) (Result, error) {
	if err := validate(cfg); err != nil {
		return Result{}, err
	}

	server := httpserver.New(logger, httpserver.Host(cfg.Host), httpserver.Port(cfg.Port), handler(logger))
	return Result{}, server.Serve(ctx, cfg.ShutdownTimeout)
}

// validate rejects out-of-range ports and too-small shutdown timeouts.
func validate(cfg Config) error {
	if cfg.Port < minPort || cfg.Port > maxPort {
		return constants.ErrInvalidValue.With(nil, "port must be between 0 and 65535")
	}
	if cfg.ShutdownTimeout < minTimeout {
		return constants.ErrInvalidValue.With(nil, "shutdown timeout must be at least 1 second")
	}
	return nil
}
