package list

import "github.com/gomatic/template.cli/internal/config"

// Config holds the flags for the "config list" command.
type Config struct {
	Prefix config.Prefix
}
