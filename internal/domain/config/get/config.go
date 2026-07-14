package get

import "github.com/gomatic/template.cli/internal/config"

// Config holds the flags for the "config get" command.
type Config struct {
	DefaultValue config.Value
}
