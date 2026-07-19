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
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
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
			cmdEval, cmdServe, cmdTUI, cmdComplete, cmdMan,
		},
		names,
	)
}

func TestCLI_Render(t *testing.T) {
	path := writeTemp(t, "s.tsvt", sampleSheet)
	out, err := runCLI(t, "render", path)
	require.NoError(t, err)
	assert.Contains(t, out, "\t5\n")
}

func TestCLI_RenderFormatDefaultsToTSV(t *testing.T) {
	// No --format → TSV: tab-separated, no commas.
	path := writeTemp(t, "s.tsvt", sampleSheet)
	out, err := runCLI(t, "render", path)
	require.NoError(t, err)
	assert.Equal(t, "2\t3\t5\n4\t5\t9\n", out)
}

func TestCLI_RenderFormatCSV(t *testing.T) {
	// --format csv threads through to the CSV serializer.
	path := writeTemp(t, "s.tsvt", sampleSheet)
	out, err := runCLI(t, "render", "--format", "csv", path)
	require.NoError(t, err)
	assert.Equal(t, "2,3,5\n4,5,9\n", out)
}

func TestCLI_RenderFormatMarkdownAlias(t *testing.T) {
	// The -f alias and the md format alias both resolve to a pipe table.
	path := writeTemp(t, "s.tsvt", sampleSheet)
	out, err := runCLI(t, "render", "-f", "md", path)
	require.NoError(t, err)
	assert.Equal(t, "| 2 | 3 | 5 |\n| --- | --- | --- |\n| 4 | 5 | 9 |\n", out)
}

func TestCLI_RenderFormatHTML(t *testing.T) {
	path := writeTemp(t, "s.tsvt", sampleSheet)
	out, err := runCLI(t, "render", "--format", "html", path)
	require.NoError(t, err)
	assert.Contains(t, out, `<table class="tsvsheet">`)
	assert.Contains(t, out, "<tr><td>2</td><td>3</td><td>5</td></tr>")
}

func TestCLI_RenderFormatUnknown(t *testing.T) {
	path := writeTemp(t, "s.tsvt", sampleSheet)
	_, err := runCLI(t, "render", "--format", "bogus", path)
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrUnknownFormat)
}

func TestCLI_DefaultCommandRenders(t *testing.T) {
	// No subcommand → the default command renders (so a shebang'd .tsvt run as
	// `tsv file.tsvt` computes it).
	path := writeTemp(t, "s.tsvt", sampleSheet)
	out, err := runCLI(t, path)
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
	assert.Contains(t, out, `"rows"`)
	assert.Contains(t, out, `=A1+B1`)      // source grid carries the formula
	assert.NotContains(t, out, `"values"`) // computed grid omitted without the flag
}

func TestCLI_ParseWithValue(t *testing.T) {
	withStdin(t, sampleSheet)
	out, err := runCLI(t, "parse", "--value")
	require.NoError(t, err)
	assert.Contains(t, out, `"values"`)
	assert.Contains(t, out, `"5"`) // C1 = A1+B1 = 5, in the computed grid
}

func TestCLI_ParseRoundTripsThroughFromJSON(t *testing.T) {
	// parse → from-json reconstructs the original source.
	json, err := runCLI(t, "parse", writeTemp(t, "s.tsvt", sampleSheet))
	require.NoError(t, err)
	withStdin(t, json)
	back, err := runCLI(t, "from-json")
	require.NoError(t, err)
	assert.Equal(t, sampleSheet, back)
}

func TestCLI_CheckClean(t *testing.T) {
	withStdin(t, sampleSheet)
	_, err := runCLI(t, cmdCheck)
	require.NoError(t, err)
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
	assert.Equal(t, exitSyntaxError, exitCode(tsvsheet.ErrSyntax.With(nil)))
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

func TestRunParse_ReadError(t *testing.T) {
	t.Parallel()

	streams := Streams{In: failReader{}, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := runParse(streams, "-", false, false, tsvsheet.DefaultLimits(), nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrReadInput)
}

func TestRunRender_ReadError(t *testing.T) {
	t.Parallel()

	streams := Streams{In: failReader{}, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := runRender(streams, "-", formatTSV, false, tsvsheet.DefaultLimits(), nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrReadInput)
}

func TestRunExplain_ReadError(t *testing.T) {
	t.Parallel()

	streams := Streams{In: failReader{}, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := runExplain(streams, explainConfig{source: "-", cell: "A1"})
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrReadInput)
}
