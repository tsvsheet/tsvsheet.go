package rename

import (
	app "github.com/gomatic/go-app"
	"github.com/urfave/cli/v3"

	domain "github.com/gomatic/template.cli/internal/domain/rename"
)

const (
	name        = `rename`
	usage       = `Rename this template's identity to a new project`
	argUsage    = `[name]`
	description = `Rewrite every template-identity token to a new project name.

Intended to be run once, immediately after cloning this template into a new
repository. The Go module path is taken from the git origin remote (because Go
requires the module path to be the repository), and every reference to the old
module path, command name, environment prefix, and package identifier is
rewritten across the tracked files; the cmd/<old> directory is moved to
cmd/<new>.

The optional [name] sets only the command/binary name (cmd/<name>); when omitted
it defaults to the repository name from the remote. The module path always
follows the remote regardless of [name].

Examples:
  template.cli rename
  template.cli rename mytool
  template.cli rename --dry-run

The --dry-run flag reports which files would change without writing or moving
anything.`
)

const dryRunFlag = "dry-run"

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
				Usage:       "Report what would change without writing or moving anything",
				Destination: (*bool)(&cfg.DryRunEnabled),
			},
		},
	}
}
