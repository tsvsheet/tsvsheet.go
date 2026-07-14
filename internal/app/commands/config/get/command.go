package get

import (
	app "github.com/gomatic/go-app"
	"github.com/urfave/cli/v3"

	domain "github.com/gomatic/template.cli/internal/domain/config/get"
)

const (
	name        = `get`
	usage       = `Get a configuration value`
	argUsage    = `<key>`
	description = `Retrieve a configuration value by key.

Examples:
  template.cli config get app.name
  template.cli config get database.host
  template.cli config get missing.key --default "fallback-value"

This command demonstrates:
  - Single positional argument
  - Optional flags with defaults
  - Error handling for missing keys`
)

const (
	defaultFlag = "default"
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
			&cli.StringFlag{
				Name:        defaultFlag,
				Aliases:     []string{"d"},
				Sources:     cli.EnvVars("CONFIG_DEFAULT"),
				Usage:       "Default value if key not found",
				Destination: (*string)(&cfg.DefaultValue),
			},
		},
	}
}
