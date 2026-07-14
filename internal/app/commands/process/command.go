package process

import (
	app "github.com/gomatic/go-app"
	"github.com/urfave/cli/v3"

	domain "github.com/gomatic/template.cli/internal/domain/process"
)

const (
	name        = `process`
	usage       = `Process text line by line with various transformations.`
	argUsage    = `[file]`
	description = `Process text input line by line, applying various transformations.

This command demonstrates:
  - Reading from stdin when no file is provided
  - Reading from a file when specified
  - Writing to stdout
  - Multiple flag types (string, bool)
  - Unix pipe composability

Examples:
  # Process from stdin
  echo "hello world" | template.cli process --uppercase

  # Process from file
  template.cli process --line-numbers input.txt

  # Chain with other Unix tools
  cat data.txt | template.cli process --filter=error | grep -i critical

  # Combine multiple transformations
  template.cli process --uppercase --prefix=">> " --line-numbers input.txt`
)

const (
	filterFlag      = "filter"
	lineNumbersFlag = "line-numbers"
	prefixFlag      = "prefix"
	uppercaseFlag   = "uppercase"
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
				Name:        uppercaseFlag,
				Aliases:     []string{"u"},
				Sources:     cli.EnvVars("UPPERCASE"),
				Usage:       "Convert all text to uppercase",
				Destination: (*bool)(&cfg.UppercaseEnabled),
			},
			&cli.BoolFlag{
				Name:        lineNumbersFlag,
				Aliases:     []string{"n"},
				Sources:     cli.EnvVars("LINE_NUMBERS"),
				Usage:       "Add line numbers to output",
				Destination: (*bool)(&cfg.LineNumbersEnabled),
			},
			&cli.StringFlag{
				Name:        prefixFlag,
				Aliases:     []string{"p"},
				Sources:     cli.EnvVars("PREFIX"),
				Usage:       "Add a prefix to each line",
				Destination: (*string)(&cfg.Prefix),
			},
			&cli.StringFlag{
				Name:        filterFlag,
				Aliases:     []string{"f"},
				Sources:     cli.EnvVars("FILTER"),
				Usage:       "Only output lines containing this text",
				Destination: (*string)(&cfg.Filter),
			},
		},
	}
}
