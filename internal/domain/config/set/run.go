package set

import (
	"context"
	"log/slog"

	"github.com/gomatic/template.cli/internal/config"
	"github.com/gomatic/template.cli/internal/domain"
	configdomain "github.com/gomatic/template.cli/internal/domain/config"
)

// Result is the outcome of the "config set" command. Setting produces no value.
type Result struct{}

// Run validates the key/value pair from args and applies it to the store,
// honoring dry-run mode.
func Run(_ context.Context, logger *slog.Logger, cfg Config, args ...domain.Argument) (Result, error) {
	key, value, err := configdomain.PairFrom(args...)
	if err != nil {
		return Result{}, err
	}

	apply(logger, cfg, key, value)
	return Result{}, nil
}

// apply writes the entry to the store, or logs the intended change in dry-run mode.
func apply(logger *slog.Logger, cfg Config, key config.Key, value config.Value) {
	store := config.NewStore()
	if bool(cfg.DryRunEnabled) {
		_, exists := store.Get(key)
		logger.Info("Dry-run: configuration unchanged.", "key", key, "value", value, "exists", exists, "dry_run", true)
		return
	}

	previous, existed := store.Set(key, value)
	logger.Info("Configuration set.", "key", key, "value", value, "previous", previous, "existed", existed)
}
