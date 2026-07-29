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
	"github.com/tsvsheet/go-tsvsheet"
)

// TestCLI_MaxCellsCap proves --max-cells narrows the OOM cap that the render
// command threads into the compute pass: with a 5-cell budget a SEQUENCE(10) is
// rejected. Nothing global is mutated, so the test is safe to run in parallel.
func TestCLI_MaxCellsCap(t *testing.T) {
	t.Parallel()

	path := writeTemp(t, "big.tsvt", "=sequence(10)\n")
	out, err := runCLI(t, "--max-cells", "5", "render", path)
	require.NoError(t, err)
	assert.Contains(t, out, "#VALUE!") // 10 cells exceeds the 5-cell cap
}

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
	assert.ElementsMatch(
		t,
		[]string{
			cmdRender, cmdParse, cmdFromJSON, cmdCheck, cmdExplain,
			cmdEval, cmdServe, cmdData, cmdTUI, cmdComplete, cmdMan,
		},
		names,
	)
}

func TestCLI_AllowAnyPaths(t *testing.T) {
	// A sheet cross-referencing an absolute path outside its own directory:
	// confined (default) refuses it (#REF!); --allow-any-paths reads it.
	ext := writeTemp(t, "ext.tsvt", "99\n")
	main := writeTemp(t, "main.tsvt", "=\""+ext+"\"!A1\n")

	confined, err := runCLI(t, "render", main)
	require.NoError(t, err)
	assert.Contains(t, confined, "#REF!")

	unconfined, err := runCLI(t, "render", "--allow-any-paths", main)
	require.NoError(t, err)
	assert.Contains(t, unconfined, "99")
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

	_, err := configureLogger(context.Background(), Command("test"))
	require.NoError(t, err)
}

func TestReadAll_Error(t *testing.T) {
	t.Parallel()

	_, err := readAll(failReader{})
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrReadInput)
}

// failReader always errors, exercising readAll's error path.
type failReader struct{}

func (failReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

// TestLoggerFlags_AreNotSharedBetweenCommands guards the fix for a data race:
// --log-level/--log-format used to write through a Destination pointing at a
// package-level LoggerConfig, so every root command in the process shared one
// word. Two commands parsed concurrently raced on it, and the later parse
// silently redefined the earlier command's logging. Run under -race, two
// parallel root commands carrying DIFFERENT values are the regression probe.
func TestLoggerFlags_AreNotSharedBetweenCommands(t *testing.T) {
	t.Parallel()

	sheet := writeTemp(t, "log.tsvt", "1\n")
	for _, level := range []string{"debug", "error"} {
		t.Run(level, func(t *testing.T) {
			t.Parallel()
			out, err := runCLI(t, "--log-level", level, "render", sheet)
			require.NoError(t, err)
			assert.Contains(t, out, "1")
		})
	}
}

// sampleSheet is a single-file spreadsheet: two data columns and a C-column
// formula summing A and B per row.
const sampleSheet = "2\t3\t=A1+B1\n4\t5\t=A2+B2\n"

// streamsWith builds Streams over the given input, capturing out and err.
func streamsWith(in string) (Streams, *bytes.Buffer, *bytes.Buffer) {
	var out, errBuf bytes.Buffer
	return Streams{In: strings.NewReader(in), Out: &out, Err: &errBuf}, &out, &errBuf
}

// writeTemp writes content to a temp file and returns its path.
func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// failWriter always fails, exercising output error paths.
type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

// TestMaxCellsFlag_ZeroKeepsTheEngineDefaults pins the sentinel the doc names:
// the flag's zero value is "unset", not "a zero-cell budget". Reading it
// literally would make every sheet exceed its limit and compute nothing.
