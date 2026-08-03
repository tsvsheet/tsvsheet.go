package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
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
	require.NoError(t, runRender(streams, "-", formatTSV, hiddenKeep, false, tsvsheet.DefaultLimits(), nil, ""))
	assert.Equal(t, "2\t3\t5\n4\t5\t9\n", out.String())
}

func TestRunRender_ReadsFile(t *testing.T) {
	t.Parallel()

	path := writeTemp(t, "s.tsvt", sampleSheet)
	streams, out, _ := streamsWith("")
	require.NoError(
		t,
		runRender(streams, sourcePath(path), formatTSV, hiddenKeep, false, tsvsheet.DefaultLimits(), nil, ""),
	)
	assert.Contains(t, out.String(), "\t5\n")
}

func TestRunRender_FileMissing(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("")
	err := runRender(streams, "/no/such.tsvt", formatTSV, hiddenKeep, false, tsvsheet.DefaultLimits(), nil, "")
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrOpenFile)
}

func TestRunRender_SyntaxError(t *testing.T) {
	t.Parallel()

	streams, _, _ := streamsWith("1\t=sum(\n")
	err := runRender(streams, "-", formatTSV, hiddenKeep, false, tsvsheet.DefaultLimits(), nil, "")
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrSyntax)
}

func TestRunRender_WriteError(t *testing.T) {
	t.Parallel()

	streams := Streams{In: strings.NewReader(sampleSheet), Out: failWriter{}, Err: &bytes.Buffer{}}
	err := runRender(streams, "-", formatTSV, hiddenKeep, false, tsvsheet.DefaultLimits(), nil, "")
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
	err := runRender(streams, "-", formatTSV, hiddenKeep, false, tsvsheet.DefaultLimits(), nil, "")
	require.Error(t, err)
	assert.ErrorIs(t, err, tsvsheet.ErrReadInput)
}

func TestRender_RelativeReferenceResolvesAgainstAScopedServer(t *testing.T) {
	t.Parallel()
	sheet := writeTemp(t, "rel.tsvt", "=importsheet(\"balances.tsv\")\n")
	out, err := runCLI(t, "render", "--data", dataDir(t), sheet)
	require.NoError(t, err)
	assert.Contains(t, out, "Brokerage\t310000")
}

func TestRender_RelativeReferenceWithoutDataIsImportError(t *testing.T) {
	t.Parallel()
	sheet := writeTemp(t, "rel.tsvt", "=importsheet(\"balances.tsv\")\n")
	out, err := runCLI(t, "render", sheet)
	require.NoError(t, err)
	assert.Contains(t, out, "#IMPORT!")
}

func TestRender_TraversalAboveTheBaseIsRefused(t *testing.T) {
	t.Parallel()
	sheet := writeTemp(t, "esc.tsvt", "=importcell(\"../../etc/hosts\")\n")
	out, err := runCLI(t, "render", "--data", dataDir(t), sheet)
	require.NoError(t, err)
	assert.Contains(t, out, "#IMPORT!")
}

func TestRender_DataDoesNotWidenTheImportAllowlist(t *testing.T) {
	t.Parallel()
	// A live server the sheet names absolutely: --data authorized a base, and a
	// base is not a host, so this must still be denied.
	server := httptest.NewServer(nil)
	t.Cleanup(server.Close)

	sheet := writeTemp(t, "abs.tsvt", "=importcell(\""+server.URL+"/x.tsv\")\n")
	out, err := runCLI(t, "render", "--data", dataDir(t), sheet)
	require.NoError(t, err)
	assert.Contains(t, out, "#IMPORT!")
}

func TestRender_BadDataFlagFailsBeforeComputing(t *testing.T) {
	t.Parallel()
	sheet := writeTemp(t, "rel.tsvt", "=importsheet(\"balances.tsv\")\n")
	_, err := runCLI(t, "render", "--data", "http://data.example.com/", sheet)
	require.ErrorIs(t, err, constants.ErrImportScheme)
}

func TestRender_LocalAndRemoteBasesComputeIdentically(t *testing.T) {
	t.Parallel()

	// A "remote" base — a server this test runs, reached by URL rather than by
	// --data starting one. From the sheet's side the two are indistinguishable.
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/tab-separated-values")
		_, _ = w.Write([]byte("Brokerage\t310000\n"))
	}))
	t.Cleanup(remote.Close)

	sheet := writeTemp(t, "portable.tsvt", "=importsheet(\"balances.tsv\")\n")

	fromDir, err := runCLI(t, "render", "--data", dataDir(t), sheet)
	require.NoError(t, err)
	fromURL, err := runCLI(t, "render", "--data", remote.URL+"/", sheet)
	require.NoError(t, err)

	assert.Equal(t, fromDir, fromURL, "the same sheet computes identically against either base")
	assert.Contains(t, fromDir, "Brokerage\t310000")
}

func TestRender_ScopedServerIsClosedOnAnErrorExit(t *testing.T) {
	t.Parallel()

	sheet := writeTemp(t, "err.tsvt", "=importsheet(\"balances.tsv\")\n")
	_, err := runCLI(t, "render", "--data", dataDir(t), "--format", "bogus", sheet)
	require.ErrorIs(t, err, constants.ErrUnknownFormat, "the command failed after the server was up")
}

// TestRender_CellPrintsOneValueVerbatim pins the emitter-sheet escape hatch:
// --cell prints exactly the computed text of one cell — embedded newlines
// intact, no tabs, no escaping — where the grid path would silently turn that
// same value into extra rows. This is the difference between a sheet that
// computes an SVG and a sheet that can hand one to the next program.
func TestRender_CellPrintsOneValueVerbatim(t *testing.T) {
	t.Parallel()

	// B1 computes a two-line value; through the grid it reads back as two rows.
	path := writeTemp(t, "emit.tsvt", "x\t=concat(\"<svg>\", char(10), \"</svg>\")\n")

	out := &bytes.Buffer{}
	require.NoError(t, runRender(Streams{Out: out}, sourcePath(path), formatTSV, hiddenKeep,
		false, tsvsheet.DefaultLimits(), nil, "B1"))
	assert.Equal(t, "<svg>\n</svg>\n", out.String(), "the cell's own bytes, plus exactly one terminator")

	grid := &bytes.Buffer{}
	require.NoError(t, runRender(Streams{Out: grid}, sourcePath(path), formatTSV, hiddenKeep,
		false, tsvsheet.DefaultLimits(), nil, ""))
	assert.Equal(t, "x\t<svg>\n</svg>\n", grid.String(),
		"without --cell the same value is tab-joined and its newline breaks the row — the reason --cell exists")
}

// TestRender_CellRefusesBadReferences pins the two ways a caller can name a
// cell that is not there: a malformed address and one past the computed grid.
// Both are usage errors with their own sentinel, never a silent empty line.
func TestRender_CellRefusesBadReferences(t *testing.T) {
	t.Parallel()

	path := writeTemp(t, "small.tsvt", "1\t2\n")
	for name, ref := range map[string]cellRef{"malformed": "nope!", "past the grid": "Z99"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out := &bytes.Buffer{}
			err := runRender(Streams{Out: out}, sourcePath(path), formatTSV, hiddenKeep,
				false, tsvsheet.DefaultLimits(), nil, ref)
			require.Error(t, err)
			assert.Empty(t, out.String(), "a refused reference writes nothing at all")
		})
	}

	out := &bytes.Buffer{}
	err := runRender(Streams{Out: out}, sourcePath(path), formatTSV, hiddenKeep,
		false, tsvsheet.DefaultLimits(), nil, "Z99")
	assert.ErrorIs(t, err, constants.ErrOutsideGrid)
}

// TestRender_CellIgnoresHiddenAndFormat pins the precedence: --cell answers
// with one value, so neither --format nor a sheet's hide directives may
// reshape or suppress it.
func TestRender_CellIgnoresHiddenAndFormat(t *testing.T) {
	t.Parallel()

	path := writeTemp(t, "hidden.tsvt", "#.hide\trows(range(1:1))\nsecret\tb\n")
	out := &bytes.Buffer{}
	require.NoError(t, runRender(Streams{Out: out}, sourcePath(path), formatCSV, hiddenDrop,
		false, tsvsheet.DefaultLimits(), nil, "A1"))
	assert.Equal(t, "secret\n", out.String(), "the named cell answers even where the viewport hides it")
}

// TestRender_CellReportsAWriteFailure pins the last leg: a stdout that fails
// mid-write surfaces as ErrWriteFile rather than a silent success, so a
// truncated `> chart.svg` is reported instead of shipped.
func TestRender_CellReportsAWriteFailure(t *testing.T) {
	t.Parallel()

	path := writeTemp(t, "one.tsvt", "value\n")
	err := runRender(Streams{Out: failWriter{}}, sourcePath(path), formatTSV, hiddenKeep,
		false, tsvsheet.DefaultLimits(), nil, "A1")
	assert.ErrorIs(t, err, tsvsheet.ErrWriteFile)
}
