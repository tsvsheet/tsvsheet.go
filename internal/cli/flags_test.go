package cli

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

func TestPositional(t *testing.T) {
	t.Parallel()

	args := positional{"first", "second"}
	assert.Equal(t, sourcePath("first"), args.at(0))
	assert.Equal(t, sourcePath("second"), args.at(1))
	assert.Equal(t, sourcePath(""), args.at(2)) // missing → stdin
	assert.Equal(t, "first", args.text(0))
	assert.Equal(t, "", args.text(5)) // missing → empty
}

func TestExitCode(t *testing.T) {
	t.Parallel()

	assert.Equal(t, exitOK, exitCode(nil))
	assert.Equal(t, exitSyntaxError, exitCode(tsvsheet.ErrSyntax.With(nil)))
	assert.Equal(t, exitError, exitCode(constants.ErrDiagnostics.With(nil)))
	assert.Equal(t, exitError, exitCode(errors.New("boom")))
}

func TestMaxCellsFlag_ZeroKeepsTheEngineDefaults(t *testing.T) {
	t.Parallel()

	flag := maxCellsFlag()
	require.NotNil(t, flag)
	assert.Equal(t, flagMaxCells, flag.Names()[0])

	out, err := runCLI(t, "render", writeTemp(t, "seq.tsvt", "=sequence(10)\n"))
	require.NoError(t, err)
	assert.NotContains(t, out, "#VALUE!", "no --max-cells means the generous defaults, not zero")
}

// TestArgsUsageSheet_EverySheetCommandDeclaresTheSameForm pins why the
// positional form is a shared constant: help output is a contract with the
// reader, and three commands spelling the same argument three ways is a
// documentation bug that no compiler catches.
func TestArgsUsageSheet_EverySheetCommandDeclaresTheSameForm(t *testing.T) {
	t.Parallel()

	root := Command("test")
	for _, name := range []string{cmdServe, cmdTUI} {
		cmd := root.Command(name)
		require.NotNil(t, cmd, name)
		assert.Equal(t, argsUsageSheet, cmd.ArgsUsage, name)
	}
}
