package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

func TestRunRender_ComputesFromStdin(t *testing.T) {
	t.Parallel()

	streams, out, _ := streamsWith(sampleSheet)
	require.NoError(t, runRender(streams, "-", formatTSV, hiddenKeep, false, tsvsheet.DefaultLimits(), nil))
	assert.Equal(t, "2\t3\t5\n4\t5\t9\n", out.String())
}

func TestRunRender_ReadsFile(t *testing.T) {
	t.Parallel()

	path := writeTemp(t, "s.tsvt", sampleSheet)
	streams, out, _ := streamsWith("")
	require.NoError(
		t,
		runRender(streams, sourcePath(path), formatTSV, hiddenKeep, false, tsvsheet.DefaultLimits(), nil),
	)
	assert.Contains(t, out.String(), "\t5\n")
}

func TestRunRender_FileMissing(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("")
	err := runRender(streams, "/no/such.tsvt", formatTSV, hiddenKeep, false, tsvsheet.DefaultLimits(), nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrOpenFile)
}

func TestRunRender_SyntaxError(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("1\t=sum(\n")
	err := runRender(streams, "-", formatTSV, hiddenKeep, false, tsvsheet.DefaultLimits(), nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrSyntax)
}

func TestRunRender_WriteError(t *testing.T) {
	t.Parallel()

	streams := Streams{In: strings.NewReader(sampleSheet), Out: failWriter{}, Err: &bytes.Buffer{}}
	err := runRender(streams, "-", formatTSV, hiddenKeep, false, tsvsheet.DefaultLimits(), nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrWriteFile)
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

func TestRunRender_ReadError(t *testing.T) {
	t.Parallel()

	streams := Streams{In: failReader{}, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	err := runRender(streams, "-", formatTSV, hiddenKeep, false, tsvsheet.DefaultLimits(), nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrReadInput)
}
