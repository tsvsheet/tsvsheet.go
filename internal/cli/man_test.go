package cli

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

// TestCLI_Man proves the man command emits a roff manual page for the whole
// CLI — the .TH title header naming the program, and every subcommand —
// dispatched through the command wiring.
func TestCLI_Man(t *testing.T) {
	out, err := runCLI(t, cmdMan)
	require.NoError(t, err)
	assert.Contains(t, out, ".TH")
	assert.Contains(t, out, name)
	for _, sub := range []string{cmdRender, cmdCheck, cmdExplain, cmdEval, cmdComplete} {
		assert.Contains(t, out, sub)
	}
}

// TestRunMan_RendererError asserts a renderer failure surfaces as the
// ErrManPage sentinel wrapping the cause.
func TestRunMan_RendererError(t *testing.T) {
	prev := manRenderer
	manRenderer = func(*cli.Command) (string, error) { return "", errors.New("boom") }
	t.Cleanup(func() { manRenderer = prev })

	err := runMan(nil, Command("test"))
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrManPage)
}

// TestRunMan_WriteError asserts a write failure surfaces as ErrManPage.
func TestRunMan_WriteError(t *testing.T) {
	t.Parallel()

	err := runMan(failWriter{}, Command("test"))
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrManPage)
}
