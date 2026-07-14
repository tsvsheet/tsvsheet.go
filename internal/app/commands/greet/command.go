package greet

import (
	app "github.com/gomatic/go-app"
	"github.com/urfave/cli/v3"

	domain "github.com/gomatic/template.cli/internal/domain/greet"
)

const (
	name        = `greet`
	usage       = `Generate a greeting message.`
	argUsage    = `<name>`
	description = `Generate a greeting message for the specified name.

This command demonstrates:
  - Required positional arguments
  - String flags with defaults and environment variables
  - Boolean flags
  - Integer flags
  - Output to stdout`
)

const (
	enthusiastFlag = "enthusiast"
	greetingFlag   = "greeting"
	repeatFlag     = "repeat"
	uppercaseFlag  = "uppercase"
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
				Name:        greetingFlag,
				Aliases:     []string{"g"},
				Sources:     cli.EnvVars("GREETING"),
				Value:       "Hello",
				Usage:       "The greeting word to use",
				Destination: (*string)(&cfg.Greeting),
			},
			&cli.BoolFlag{
				Name:        uppercaseFlag,
				Aliases:     []string{"u"},
				Sources:     cli.EnvVars("UPPERCASE"),
				Usage:       "Convert greeting to uppercase",
				Destination: (*bool)(&cfg.UppercaseEnabled),
			},
			&cli.IntFlag{
				Name:        repeatFlag,
				Aliases:     []string{"r"},
				Sources:     cli.EnvVars("REPEAT"),
				Value:       1,
				Usage:       "Number of times to repeat the greeting",
				Destination: (*int)(&cfg.Repeat),
			},
			&cli.BoolFlag{
				Name:        enthusiastFlag,
				Aliases:     []string{"e"},
				Sources:     cli.EnvVars("ENTHUSIAST"),
				Usage:       "Add extra enthusiasm (extra exclamation marks)",
				Destination: (*bool)(&cfg.EnthusiastEnabled),
			},
		},
	}
}
