package get

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gomatic/template.cli/internal/config"
	"github.com/gomatic/template.cli/internal/constants"
	"github.com/gomatic/template.cli/internal/domain"
	configdomain "github.com/gomatic/template.cli/internal/domain/config"
)

// Result is the outcome of the "config get" command.
type Result struct {
	Value config.Value `json:"value"`
}

// Run resolves the configuration value for the key named in args, falling back
// to cfg.DefaultValue when the key is absent.
func Run(_ context.Context, logger *slog.Logger, cfg Config, args ...domain.Argument) (Result, error) {
	key, err := configdomain.KeyFrom(args...)
	if err != nil {
		return Result{}, err
	}

	value, err := resolve(logger, cfg, key)
	if err != nil {
		return Result{}, err
	}

	return Result{Value: value}, nil
}

// resolve reads the key from the store, applying the default when it is absent.
func resolve(logger *slog.Logger, cfg Config, key config.Key) (config.Value, error) {
	if value, ok := config.NewStore().Get(key); ok {
		logger.Info("Config value retrieved.", "key", key)
		return value, nil
	}
	if cfg.DefaultValue != "" {
		logger.Info("Key not found, using default.", "key", key, "default", cfg.DefaultValue)
		return cfg.DefaultValue, nil
	}
	return "", constants.ErrNotFound.With(nil, fmt.Sprintf("key %q not found", key))
}
