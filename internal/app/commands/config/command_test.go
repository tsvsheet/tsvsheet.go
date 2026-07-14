package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommand(t *testing.T) {
	t.Parallel()
	want, must := assert.New(t), require.New(t)

	command := Command()
	want.Equal("config", command.Name)
	must.NotEmpty(command.Commands, "config should expose subcommands")

	names := make([]string, 0, len(command.Commands))
	for _, sub := range command.Commands {
		names = append(names, sub.Name)
	}
	want.ElementsMatch([]string{"get", "list", "set"}, names)
}
