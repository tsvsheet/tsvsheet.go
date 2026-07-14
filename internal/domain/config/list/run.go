package list

import (
	"context"
	"log/slog"

	"github.com/gomatic/template.cli/internal/config"
	"github.com/gomatic/template.cli/internal/domain"
)

// Result is the set of configuration entries returned by the command. As a
// named map type it marshals to a JSON object of key/value pairs.
type Result map[config.Key]config.Value

// Run returns the configuration entries matching the optional prefix.
func Run(_ context.Context, logger *slog.Logger, cfg Config, _ ...domain.Argument) (Result, error) {
	matches := config.NewStore().List(cfg.Prefix)
	logger.Info("Config values listed.", "count", len(matches), "prefix", cfg.Prefix)
	return matches, nil
}
