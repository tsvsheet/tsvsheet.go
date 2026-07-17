package cli

import (
	"context"
	"slices"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/uplang/tsvsheet.go/internal/constants"
)

// shellName is a shell the completion command can emit a script for. It is the
// positional argument to `tsvsheet completion <shell>`.
type shellName string

// The shells for which tsvsheet emits a completion script, in help/message
// order. Each maps to a subcommand of urfave/cli's built-in (renamed) shell
// completion command, whose per-shell script templates are reused verbatim.
const (
	shellBash shellName = "bash"
	shellZsh  shellName = "zsh"
	shellFish shellName = "fish"
)

// supportedShells is the ordered set of shells the completion command accepts.
var supportedShells = []shellName{shellBash, shellZsh, shellFish}

// supported reports whether s is a shell the completion command emits a script
// for.
func (s shellName) supported() bool { return slices.Contains(supportedShells, s) }

// supportedShellList renders the supported shells as a comma-separated string
// for the diagnostic naming them.
func supportedShellList() string {
	names := make([]string, len(supportedShells))
	for i, s := range supportedShells {
		names[i] = string(s)
	}
	return strings.Join(names, ", ")
}

// runCompletion writes the shell completion script for the requested shell to
// the root command's writer, delegating the script generation to urfave/cli's
// built-in per-shell templates. A missing shell is ErrMissingArgument (naming
// the supported shells); an unrecognized shell is ErrUnsupportedShell.
func runCompletion(ctx context.Context, root *cli.Command, shell shellName) error {
	if shell == "" {
		return constants.ErrMissingArgument.With(nil, "argument", "shell", "supported", supportedShellList())
	}
	renderer := completionRenderer(root, shell)
	if renderer == nil {
		return constants.ErrUnsupportedShell.With(nil, "shell", string(shell), "supported", supportedShellList())
	}
	return renderer.Action(ctx, renderer)
}

// completionRenderer resolves the built-in per-shell completion subcommand for a
// supported shell, or nil for an unsupported one. The built-in command is added
// under builtinCompletionName by EnableShellCompletion, so a supported shell
// always resolves to a non-nil renderer.
func completionRenderer(root *cli.Command, shell shellName) *cli.Command {
	if !shell.supported() {
		return nil
	}
	return root.Command(builtinCompletionName).Command(string(shell))
}

// completionCommand builds the `completion` command: it prints the shell
// completion script for the requested shell so a user can source it.
func completionCommand() *cli.Command {
	return &cli.Command{
		Name:      cmdComplete,
		Usage:     "Print a shell completion script for bash, zsh, or fish.",
		ArgsUsage: "<shell>",
		Description: `Print the shell completion script for the named shell (bash, zsh, or fish)
to stdout. Source the output to enable completion of tsvsheet's commands and
flags.

Examples:
  source <(tsvsheet completion bash)                            # .bashrc
  source <(tsvsheet completion zsh)                             # .zshrc
  tsvsheet completion fish > ~/.config/fish/completions/tsvsheet.fish`,
		Action: func(ctx context.Context, c *cli.Command) error {
			return runCompletion(ctx, c.Root(), shellName(c.Args().First()))
		},
	}
}
