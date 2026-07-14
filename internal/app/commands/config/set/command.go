package set

import (
	app "github.com/gomatic/go-app"
	"github.com/urfave/cli/v3"

	domain "github.com/gomatic/template.cli/internal/domain/config/set"
)

const (
	name        = `set`
	usage       = `Set a configuration value`
	argUsage    = `<key> <value>`
	description = `Update or create a configuration value.

Examples:
  template.cli config set app.name myapp
  template.cli config set database.host localhost
  template.cli config set log.level debug --dry-run

The --dry-run flag shows what would be changed without making the change.
This is a standard Unix pattern (-n/--dry-run) for safe operation previews.

This command demonstrates:
  - Multiple positional arguments (key and value)
  - Dry-run flag pattern
  - Update vs create distinction in logging`
)

const (
	dryRunFlag = "dry-run"
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
		ArgsUsage:   argUsage,
		Description: description,
		Action:      app.Default(&cfg, runAction),
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        dryRunFlag,
				Aliases:     []string{"n"},
				Sources:     cli.EnvVars("DRY_RUN"),
				Usage:       "Show what would be done without making changes",
				Destination: (*bool)(&cfg.DryRunEnabled),
			},
		},
	}
}
