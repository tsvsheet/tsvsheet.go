package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uplang/go-tsvsheet"

	"github.com/uplang/tsvsheet.go/internal/constants"
)

// computedGrid is a representative computed value grid (two numeric rows) used
// to pin each format's exact serialization.
var computedGrid = tsvsheet.Grid{{"2", "3", "5"}, {"4", "5", "9"}}

func TestFormat_Serializations(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		f    Format
		want string
		grid tsvsheet.Grid
	}{
		{
			// The default path is byte-identical to tsvsheet.WriteTSV.
			name: "tsv",
			f:    formatTSV,
			grid: computedGrid,
			want: "2\t3\t5\n4\t5\t9\n",
		},
		{
			// RFC 4180: a cell with a comma is quoted, an embedded quote is
			// doubled, and an embedded newline is quoted.
			name: "csv quoting",
			f:    formatCSV,
			grid: tsvsheet.Grid{{"a", "b,c"}, {"d\"e", "f\ng"}},
			want: "a,\"b,c\"\n\"d\"\"e\",\"f\ng\"\n",
		},
		{
			// Every cell is HTML-escaped (< > &), plain <table>/<tr>/<td>.
			name: "html escaping",
			f:    formatHTML,
			grid: tsvsheet.Grid{{"<b>", "a&b"}, {"x", "y"}},
			want: "<table class=\"tsvsheet\">\n" +
				"<tr><td>&lt;b&gt;</td><td>a&amp;b</td></tr>\n" +
				"<tr><td>x</td><td>y</td></tr>\n" +
				"</table>\n",
		},
		{
			// Header row, --- separator, body; a | in a cell is escaped as \|.
			name: "markdown escaping",
			f:    formatMarkdown,
			grid: tsvsheet.Grid{{"H1", "H2"}, {"a|b", "c"}},
			want: "| H1 | H2 |\n| --- | --- |\n| a\\|b | c |\n",
		},
		{
			// md is an alias for markdown.
			name: "md alias",
			f:    formatMD,
			grid: tsvsheet.Grid{{"H1", "H2"}, {"a", "b"}},
			want: "| H1 | H2 |\n| --- | --- |\n| a | b |\n",
		},
		{
			// A single-row grid is still a valid table: header + separator, no
			// body rows.
			name: "markdown single row",
			f:    formatMarkdown,
			grid: tsvsheet.Grid{{"only", "row"}},
			want: "| only | row |\n| --- | --- |\n",
		},
		{
			// An empty grid yields no Markdown output.
			name: "markdown empty",
			f:    formatMarkdown,
			grid: tsvsheet.Grid{},
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			require.NoError(t, format(&out, tc.grid, tc.f))
			assert.Equal(t, tc.want, out.String())
		})
	}
}

func TestFormat_Unknown(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := format(&out, computedGrid, "xml")
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrUnknownFormat)
	assert.Contains(t, err.Error(), "xml") // the offending value is named
	assert.Empty(t, out.String())
}

func TestFormat_WriteErrors(t *testing.T) {
	t.Parallel()

	// Each formatter surfaces an output failure as ErrWriteFile.
	for _, f := range []Format{formatCSV, formatHTML, formatMarkdown} {
		t.Run(string(f), func(t *testing.T) {
			t.Parallel()

			err := format(failWriter{}, computedGrid, f)
			require.Error(t, err)
			assert.ErrorIs(t, err, tsvsheet.ErrWriteFile)
		})
	}
}
