package config

import (
	"github.com/urfave/cli/v3"

	"github.com/gomatic/template.cli/internal/app/commands/config/get"
	"github.com/gomatic/template.cli/internal/app/commands/config/list"
	"github.com/gomatic/template.cli/internal/app/commands/config/set"
)

const (
	name        = `config`
	usage       = `Manage configuration settings`
	description = `Manage configuration settings for the application.

This is a parent command that demonstrates hierarchical command structure
following the "noun...verb" pattern (resource-oriented design):

  config get <key>         - Retrieve a configuration value
  config set <key> <value> - Update a configuration value
  config list              - List all configuration values

This pattern makes commands discoverable and predictable. Users naturally
think "what am I operating on" (config) then "what action" (get/set/list).`
)

// Command returns the parent CLI command definition.
// Parent commands have Subcommands, not Action.
func Command() *cli.Command {
	return &cli.Command{
		Name:        name,
		Usage:       usage,
		Description: description,
		Commands: []*cli.Command{
			get.Command(),
			list.Command(),
			set.Command(),
		},
	}
}
