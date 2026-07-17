package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/constants"
)

// TestCLI_Completion proves each supported shell emits a non-empty completion
// script that names the program, dispatched through the command wiring.
func TestCLI_Completion(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		t.Run(shell, func(t *testing.T) {
			out, err := runCLI(t, cmdComplete, shell)
			require.NoError(t, err)
			assert.NotEmpty(t, out)
			assert.Contains(t, out, name)
		})
	}
}

// TestCLI_CompletionUnsupported proves a shell tsvsheet does not emit for — even
// one urfave/cli itself supports (powershell) — is the specific
// ErrUnsupportedShell sentinel, not a crash or a raw library error.
func TestCLI_CompletionUnsupported(t *testing.T) {
	for _, shell := range []string{"powershell", "pwsh", "tcsh", "nonsense"} {
		t.Run(shell, func(t *testing.T) {
			_, err := runCLI(t, cmdComplete, shell)
			require.Error(t, err)
			assert.ErrorIs(t, err, constants.ErrUnsupportedShell)
		})
	}
}

// TestCLI_CompletionMissingShell proves omitting the shell argument is the
// ErrMissingArgument sentinel, whose diagnostic names the supported shells.
func TestCLI_CompletionMissingShell(t *testing.T) {
	_, err := runCLI(t, cmdComplete)
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrMissingArgument)
	assert.Contains(t, err.Error(), supportedShellList())
}

// TestCompletionEnabledOnRoot proves the root command enables shell completion
// (the <TAB> integration) and renames urfave/cli's built-in completion command
// aside so it does not collide with tsvsheet's own `completion` command.
func TestCompletionEnabledOnRoot(t *testing.T) {
	t.Parallel()

	cmd := Command("v1")
	assert.True(t, cmd.EnableShellCompletion)
	assert.Equal(t, builtinCompletionName, cmd.ShellCompletionCommandName)
}

// TestSupportedShellList proves the diagnostic lists every supported shell in
// order.
func TestSupportedShellList(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "bash, zsh, fish", supportedShellList())
}
