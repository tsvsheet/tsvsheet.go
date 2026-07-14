package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
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
	assert.ElementsMatch(t, []string{"render", "parse", "check", "explain", "serve", "tui"}, names)
}

func TestCLI_Render(t *testing.T) {
	dataPath := filepath.Join(t.TempDir(), "d.tsv")
	require.NoError(t, os.WriteFile(dataPath, []byte(sampleData), 0o600))
	withStdin(t, sampleTemplate)

	out, err := runCLI(t, "render", "--data", dataPath)
	require.NoError(t, err)
	assert.Contains(t, out, "\t7\n")
}

func TestCLI_Parse(t *testing.T) {
	withStdin(t, "=body\nE=C + D\n")
	out, err := runCLI(t, "parse")
	require.NoError(t, err)
	assert.Contains(t, out, `"kind": "body"`)
}

func TestCLI_CheckClean(t *testing.T) {
	withStdin(t, "=body\nE=C + D\n")
	_, err := runCLI(t, "check")
	require.NoError(t, err)
}

func TestCLI_ExplainCell(t *testing.T) {
	t.Parallel()

	dataPath := filepath.Join(t.TempDir(), "d.tsv")
	require.NoError(t, os.WriteFile(dataPath, []byte(sampleData), 0o600))
	tmplPath := filepath.Join(t.TempDir(), "t.tsvt")
	require.NoError(t, os.WriteFile(tmplPath, []byte("=body\nE=C + D\n"), 0o600))

	out, err := runCLI(t, "explain", "--cell", "E2", "--template", tmplPath, "--data", dataPath)
	require.NoError(t, err)
	assert.Contains(t, out, "E2 = 9")
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

	withStdin(t, "=sum(")
	assert.Equal(t, exitSyntaxError, Run(context.Background(), []string{name, "check"}))
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
	err := runParse(streams, parseConfig{template: "-"})
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrReadInput)
}

func TestRunRender_DataReadError(t *testing.T) {
	t.Parallel()

	tmplPath := filepath.Join(t.TempDir(), "t.tsvt")
	require.NoError(t, os.WriteFile(tmplPath, []byte(sampleTemplate), 0o600))

	streams := Streams{In: failReader{}, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := runRender(streams, renderConfig{template: sourcePath(tmplPath), data: "-"})
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrReadInput)
}

func TestRunExplain_DataReadError(t *testing.T) {
	t.Parallel()

	tmplPath := filepath.Join(t.TempDir(), "t.tsvt")
	require.NoError(t, os.WriteFile(tmplPath, []byte("=body\nE=C\n"), 0o600))

	streams := Streams{In: failReader{}, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := runExplain(streams, explainConfig{template: sourcePath(tmplPath), data: "-", cell: "A1"})
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrReadInput)
}

func TestStubs_NotImplemented(t *testing.T) {
	t.Parallel()

	assert.ErrorIs(t, runServe(Streams{}, serveConfig{}), constants.ErrUnsupported)
	assert.ErrorIs(t, runTUI(Streams{}, tuiConfig{}), constants.ErrUnsupported)
}
