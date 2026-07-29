package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

// TestMissingArgument_ShowsHelpInsteadOfLoggingAnError pins the contract for
// every command with a required positional: omitting it prints that command's
// own help — which names the argument and shows a worked example — and exits
// non-zero. A log line would tell the reader only that something is missing,
// which they already know, and would put a usage mistake into the log stream
// that real failures share.
func TestMissingArgument_ShowsHelpInsteadOfLoggingAnError(t *testing.T) {
	for _, cmd := range []string{cmdData, cmdExplain, cmdComplete, cmdServe, cmdTUI} {
		t.Run(cmd, func(t *testing.T) {
			withStdin(t, "")

			out, err := runCLI(t, cmd)
			require.ErrorIs(t, err, constants.ErrUsage, "a usage mistake, not a runtime failure")
			assert.Contains(t, out, "USAGE:", "the command's own help is printed")
			assert.Contains(t, out, cmd, "and it is the help for THIS command")
			assert.Equal(t, exitSyntaxError, exitCode(err), "exits non-zero")
		})
	}
}

// TestMissingArgument_IsNotLogged pins the other half: the top level must not
// log a usage error, because its help has already been shown.
func TestMissingArgument_IsNotLogged(t *testing.T) {
	prev := stderr
	var logged bytes.Buffer
	stderr = &logged
	t.Cleanup(func() { stderr = prev })

	withStdin(t, "")
	cmd := Command("test")
	cmd.Writer = &bytes.Buffer{}
	err := cmd.Run(context.Background(), []string{name, cmdData})
	require.ErrorIs(t, err, constants.ErrUsage)

	assert.Equal(t, exitSyntaxError, exitCode(err))
	assert.NotContains(t, logged.String(), "level=ERROR", "nothing is logged for a usage mistake")
}

// TestEval_MissingExpressionShowsHelpEvenFromStdin covers the deep case: eval's
// expression may arrive on stdin, so it is not known to be absent until stdin
// has been read — the answer is still the help.
func TestEval_MissingExpressionShowsHelpEvenFromStdin(t *testing.T) {
	withStdin(t, "   \n")

	out, err := runCLI(t, cmdEval)
	require.ErrorIs(t, err, constants.ErrUsage)
	assert.Contains(t, out, "USAGE:")
}
