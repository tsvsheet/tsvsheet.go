package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

// TestCLI_Man proves the man command emits a well-formed roff manual page for
// the whole CLI — title header, NAME, SYNOPSIS, DESCRIPTION, GLOBAL OPTIONS,
// and one .SS subsection per subcommand — dispatched through the command
// wiring.
func TestCLI_Man(t *testing.T) {
	out, err := runCLI(t, cmdMan)
	require.NoError(t, err)
	assert.Contains(t, out, `.TH TSV 1 "" "tsv`)
	assert.Contains(t, out, ".SH NAME\ntsv \\- A spreadsheet in plain text")
	assert.Contains(t, out, ".SH SYNOPSIS")
	assert.Contains(t, out, ".SH DESCRIPTION")
	assert.Contains(t, out, ".SH GLOBAL OPTIONS")
	assert.Contains(t, out, ".SH COMMANDS")
	for _, sub := range []string{cmdRender, cmdParse, cmdFromJSON, cmdCheck, cmdExplain, cmdEval, cmdServe, cmdTUI, cmdComplete, cmdMan} {
		assert.Contains(t, out, ".SS \""+sub+"\"\n.B tsv "+sub)
	}
}

// TestCLI_Man_Formatting pins the formatting contract details: flags are .TP
// tagged paragraphs carrying their defaults, indented example lines ride in
// .EX blocks, and the injected help command/flag noise is absent.
func TestCLI_Man_Formatting(t *testing.T) {
	out, err := runCLI(t, cmdMan)
	require.NoError(t, err)
	assert.Contains(t, out, ".TP\n\\fB\\-\\-format\\fR, \\fB\\-f\\fR=\\fIvalue\\fR\n")
	assert.Contains(t, out, `(default: "text")`)
	assert.Contains(t, out, ".EX\n  tsv render sheet.tsvt | column -t\n")
	assert.Contains(t, out, ".EE\n")
	assert.NotContains(t, out, "Shows a list of commands")
	assert.NotContains(t, out, `.SS "help`)
	assert.NotContains(t, out, `\-\-help`)
}

// TestManPage_RootArgsSynopsis proves a root command that runs directly
// (ArgsUsage set) gets its own SYNOPSIS line ahead of the subcommand form.
func TestManPage_RootArgsSynopsis(t *testing.T) {
	t.Parallel()

	page := manPage(&cli.Command{Name: "app", ArgsUsage: "<thing> [file]"})
	assert.Contains(t, page, ".B app\n[\\fIglobal options\\fR] <thing> [file]\n.br\n")
}

// TestRoffLine pins the roff escaping contract: backslashes become printable
// \e, and lines that would read as roff requests are neutralized with \&.
func TestRoffLine(t *testing.T) {
	t.Parallel()

	assert.Equal(t, `a \e b`, roffLine(`a \ b`))
	assert.Equal(t, `\&.request`, roffLine(".request"))
	assert.Equal(t, `\&  .indented`, roffLine("  .indented"))
	assert.Equal(t, `\&'quote`, roffLine("'quote"))
	assert.Equal(t, "plain", roffLine("plain"))
}

// TestFlagDefault covers the default-text resolution: hidden defaults render
// nothing, an explicit DefaultText wins, a value-taking flag falls back to its
// initial value, and a bool flag shows no default.
func TestFlagDefault(t *testing.T) {
	t.Parallel()

	assert.Empty(t, flagDefault(&cli.StringFlag{Name: "a", Value: "x", HideDefault: true}))
	assert.Equal(t, "why", flagDefault(&cli.StringFlag{Name: "a", Value: "x", DefaultText: "why"}))
	assert.Equal(t, `"x"`, flagDefault(&cli.StringFlag{Name: "a", Value: "x"}))
	assert.Empty(t, flagDefault(&cli.BoolFlag{Name: "a"}))
}

// bareFlag implements cli.Flag but not cli.DocGenerationFlag, proving the
// documentation filter drops flags that cannot describe themselves.
type bareFlag struct{}

func (bareFlag) String() string           { return "" }
func (bareFlag) Get() any                 { return nil }
func (bareFlag) PreParse() error          { return nil }
func (bareFlag) PostParse() error         { return nil }
func (bareFlag) Set(string, string) error { return nil }
func (bareFlag) Names() []string          { return []string{"bare"} }
func (bareFlag) IsSet() bool              { return false }

// TestDocumentedFlags proves the filter keeps documentable flags and drops
// the injected help flag and non-documentable implementations.
func TestDocumentedFlags(t *testing.T) {
	t.Parallel()

	flags := []cli.Flag{
		&cli.BoolFlag{Name: "help", Aliases: []string{"h"}},
		bareFlag{},
		&cli.StringFlag{Name: "keep"},
	}
	docs := documentedFlags(flags)
	require.Len(t, docs, 1)
	assert.Equal(t, []string{"keep"}, docs[0].names)
}

// TestManProse covers the block transitions: paragraph, example, blank
// separator, and the example-to-paragraph return edge.
func TestManProse(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		".PP\npara one\ncontinued\n.EX\n  example\n.EE\n.PP\nback to prose\n.EX\n  trailing example\n.EE\n",
		manProse("para one\ncontinued\n\n  example\nback to prose\n\n  trailing example"))
}

// TestRunMan_WriteError asserts a write failure surfaces as ErrManPage.
func TestRunMan_WriteError(t *testing.T) {
	t.Parallel()

	err := runMan(failWriter{}, Command("test"))
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrManPage)
}
