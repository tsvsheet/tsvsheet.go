package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/constants"
)

// withStdin swaps the package stdin for the duration of a test.
func withStdin(t *testing.T, in string) {
	t.Helper()
	prev := stdin
	stdin = strings.NewReader(in)
	t.Cleanup(func() { stdin = prev })
}

// runCLI runs the root command with args, capturing stdout.
func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := Command("test")
	var out bytes.Buffer
	cmd.Writer = &out
	err := cmd.Run(context.Background(), append([]string{name}, args...))
	return out.String(), err
}

func TestCommand_HasAllCommands(t *testing.T) {
	t.Parallel()

	cmd := Command("v1")
	assert.Equal(t, name, cmd.Name)
	assert.Equal(t, "v1", cmd.Version)

	names := make([]string, len(cmd.Commands))
	for i, c := range cmd.Commands {
		names[i] = c.Name
	}
	assert.ElementsMatch(t, []string{cmdRender, cmdParse, cmdCheck, cmdExplain, cmdServe, cmdTUI}, names)
}

func TestCLI_Render(t *testing.T) {
	path := writeTemp(t, "s.tsvt", sampleSheet)
	out, err := runCLI(t, "render", path)
	require.NoError(t, err)
	assert.Contains(t, out, "\t5\n")
}

func TestCLI_RenderStdin(t *testing.T) {
	withStdin(t, sampleSheet)
	out, err := runCLI(t, "render") // omitted sheet → stdin
	require.NoError(t, err)
	assert.Contains(t, out, "\t9\n")
}

func TestCLI_Parse(t *testing.T) {
	withStdin(t, sampleSheet)
	out, err := runCLI(t, "parse")
	require.NoError(t, err)
	assert.Contains(t, out, `"formula": true`)
	assert.NotContains(t, out, `"value"`) // no computed value without the flag
}

func TestCLI_ParseWithValue(t *testing.T) {
	withStdin(t, sampleSheet)
	out, err := runCLI(t, "parse", "--value")
	require.NoError(t, err)
	assert.Contains(t, out, `"value": "5"`) // D2 = B2+C2 = 5, included by --value
}

func TestCLI_CheckClean(t *testing.T) {
	withStdin(t, sampleSheet)
	_, err := runCLI(t, cmdCheck)
	require.NoError(t, err)
}

func TestCLI_ExplainCell(t *testing.T) {
	path := writeTemp(t, "s.tsvt", sampleSheet)
	out, err := runCLI(t, "explain", "C1", path)
	require.NoError(t, err)
	assert.Contains(t, out, "C1 = 5")
}

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
	assert.Equal(t, exitSyntaxError, exitCode(constants.ErrSyntax.With(nil)))
	assert.Equal(t, exitError, exitCode(constants.ErrDiagnostics.With(nil)))
	assert.Equal(t, exitError, exitCode(errors.New("boom")))
}

func TestRun_ExitCodes(t *testing.T) {
	prevStderr := stderr
	stderr = io.Discard
	t.Cleanup(func() { stderr = prevStderr })

	withStdin(t, "1\t=sum(\n")
	assert.Equal(t, exitSyntaxError, Run(context.Background(), "test", []string{name, cmdCheck}))
}

func TestConfigureLogger(t *testing.T) {
	prevStderr := stderr
	stderr = io.Discard
	t.Cleanup(func() { stderr = prevStderr })

	_, err := configureLogger(context.Background(), nil)
	require.NoError(t, err)
}

func TestReadAll_Error(t *testing.T) {
	t.Parallel()

	_, err := readAll(failReader{})
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrReadInput)
}

// failReader always errors, exercising readAll's error path.
type failReader struct{}

func (failReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestRunParse_ReadError(t *testing.T) {
	t.Parallel()

	streams := Streams{In: failReader{}, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := runParse(streams, "-", false)
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrReadInput)
}

func TestRunRender_ReadError(t *testing.T) {
	t.Parallel()

	streams := Streams{In: failReader{}, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := runRender(streams, "-")
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrReadInput)
}

func TestRunExplain_ReadError(t *testing.T) {
	t.Parallel()

	streams := Streams{In: failReader{}, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := runExplain(streams, explainConfig{source: "-", cell: "A1"})
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrReadInput)
}
