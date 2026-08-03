package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

func TestRunCheck_Clean(t *testing.T) {
	t.Parallel()

	streams, _, errBuf := streamsWith(sampleSheet)
	require.NoError(t, runCheck(streams, "-", tsvsheet.DefaultLimits()))
	assert.Empty(t, errBuf.String())
}

func TestRunCheck_Diagnostics(t *testing.T) {
	t.Parallel()

	streams, _, errBuf := streamsWith("=bogus(A1)\n")
	err := runCheck(streams, "-", tsvsheet.DefaultLimits())
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrDiagnostics)
	assert.Contains(t, errBuf.String(), "A1: unknown function: bogus")
}

func TestRunCheck_SyntaxError(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("1\t=sum(\n")
	err := runCheck(streams, "-", tsvsheet.DefaultLimits())
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrSyntax)
}

func TestRunCheck_FileMissing(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("")
	err := runCheck(streams, "/no/such.tsvt", tsvsheet.DefaultLimits())
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrOpenFile)
}

func TestCLI_CheckClean(t *testing.T) {
	withStdin(t, sampleSheet)
	_, err := runCLI(t, cmdCheck)
	require.NoError(t, err)
}
