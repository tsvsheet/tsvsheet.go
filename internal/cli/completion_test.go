package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
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

// TestCLI_CompletionUnsupported proves a shell tsv does not emit for — even
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
	out, err := runCLI(t, cmdComplete)
	require.ErrorIs(t, err, constants.ErrUsage)
	// The help names the shells, which is what the old error text carried and
	// what the reader actually needs in order to retry.
	assert.Contains(t, out, "USAGE:")
	assert.Contains(t, out, "bash")
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

// TestCompletionRenderer_EverySupportedShellResolves pins the coupling the doc
// names: the renderer comes from urfave's built-in completion command, added
// under builtinCompletionName by EnableShellCompletion. If that wiring were
// ever dropped, every supported shell would silently resolve to nil and emit
// nothing — a completion script that is empty rather than absent.
func TestCompletionRenderer_EverySupportedShellResolves(t *testing.T) {
	t.Parallel()

	// The built-in command is attached while the root runs, so the invariant is
	// exercised through a real invocation rather than a bare construction.
	for _, shell := range supportedShells {
		out, err := runCLI(t, cmdComplete, string(shell))
		require.NoError(t, err, string(shell))
		assert.NotEmpty(t, out, "a supported shell emits a script, never an empty one")
	}

	_, err := runCLI(t, cmdComplete, "csh")
	require.Error(t, err, "an unsupported shell is refused rather than emitting nothing")
	assert.Nil(t, completionRenderer(Command("test"), "csh"), "and resolves to no renderer")
}

// TestRunCompletion_NeverSeesAnOmittedShell pins the split of responsibility
// the doc claims: the command answers an omitted shell with its own help, so
// the runner only ever handles a shell that was actually named — supported or
// not.
func TestRunCompletion_NeverSeesAnOmittedShell(t *testing.T) {
	out, err := runCLI(t, cmdComplete)
	require.ErrorIs(t, err, constants.ErrUsage, "the command stopped it, not the runner")
	assert.Contains(t, out, "USAGE:")

	// A shell that WAS named still reaches the runner and is judged there.
	_, err = runCLI(t, cmdComplete, "csh")
	require.ErrorIs(t, err, constants.ErrUnsupportedShell)
}
