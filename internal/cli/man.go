package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

// runMan renders the whole CLI's man page (roff) to w. Packaging writes it to
// tsv.1; a human can read it directly with `tsv man | man -l -`.
func runMan(w io.Writer, root *cli.Command) error {
	if _, err := io.WriteString(w, manPage(root)); err != nil {
		return constants.ErrManPage.With(err)
	}
	return nil
}

// manCommand builds the `man` command: it prints the CLI's man page in roff
// form, generated from the same command tree that serves --help.
func manCommand() *cli.Command {
	return &cli.Command{
		Name:  cmdMan,
		Usage: "Print the man page (roff) to stdout.",
		Description: `Print the manual page for the whole CLI in roff form, generated from the
same command tree that serves --help.

Examples:
  tsv man | man -l -        # read it now
  tsv man > tsv.1           # what packagers install as man1/tsv.1`,
		Action: func(_ context.Context, c *cli.Command) error {
			return runMan(c.Root().Writer, c.Root())
		},
	}
}

// manText is a block of hand-authored help text destined for the man page.
type manText string

// manLine is one line of man-page text.
type manLine string

// manHeading names an .SH section.
type manHeading string

// binName is the program name subcommands are invoked as.
type binName string

// helpName is the name of the help flag urfave injects; the man page omits it
// — it documents the help system, not the program.
const helpName = "help"

// manPage renders root as a complete section-1 manual page in classic roff:
// .SH sections, one .SS subsection per command, .TP tagged flags, and indented
// example lines preserved verbatim in .EX/.EE blocks.
func manPage(root *cli.Command) string {
	// An .SH starts a paragraph itself, so the description's leading .PP is
	// redundant (mandoc warns about it) — drop it.
	description := strings.TrimPrefix(manProse(manText(root.Description)), ".PP\n")
	return manHeader(root) +
		manSynopsis(root) +
		".SH DESCRIPTION\n" + description +
		manFlagsSection("GLOBAL OPTIONS", root.VisibleFlags()) +
		manCommandsSection(root)
}

// manHeader renders the .TH title line and the NAME section. The date field is
// left empty deliberately: the page is regenerated from the command tree on
// every build, and a stamped date would make the output nondeterministic.
func manHeader(root *cli.Command) string {
	source := root.Name
	if root.Version != "" {
		source += " " + root.Version
	}
	title := fmt.Sprintf(".TH %s 1 \"\" %q \"User Commands\"\n", strings.ToUpper(root.Name), source)
	return title + ".SH NAME\n" + root.Name + " \\- " + roffLine(manLine(strings.TrimSuffix(root.Usage, "."))) + "\n"
}

// manSynopsis renders the SYNOPSIS: the root's own argument form when the root
// command runs directly (ArgsUsage set), always followed by the subcommand
// form.
func manSynopsis(root *cli.Command) string {
	s := ".SH SYNOPSIS\n"
	if root.ArgsUsage != "" {
		s += ".B " + root.Name + "\n[\\fIglobal options\\fR] " + roffLine(manLine(root.ArgsUsage)) + "\n.br\n"
	}
	return s + ".B " + root.Name + "\n[\\fIglobal options\\fR] \\fIcommand\\fR [\\fIcommand options\\fR] [\\fIargument\\fR ...]\n"
}

// manProse renders hand-authored text: blank-line-separated paragraphs refill
// under .PP, and runs of indented lines (examples) are kept verbatim between
// .EX and .EE.
func manProse(text manText) string {
	lines := strings.Split(string(text), "\n")
	chunks := make([]string, 0, len(lines)+1)
	st := proseBreak
	for _, line := range lines {
		var chunk string
		chunk, st = proseLine(st, manLine(line))
		chunks = append(chunks, chunk)
	}
	return strings.Join(append(chunks, proseClose(st)), "")
}

// proseState tracks which block the prose renderer is inside.
type proseState int

const (
	proseBreak   proseState = iota // between blocks
	proseText                      // inside a filled .PP paragraph
	proseExample                   // inside an .EX literal block
)

// proseLine advances the prose state machine by one input line, returning the
// roff to emit and the next state.
func proseLine(st proseState, line manLine) (string, proseState) {
	if strings.TrimSpace(string(line)) == "" {
		return proseClose(st), proseBreak
	}
	if isIndented(line) {
		return proseOpen(st, proseExample) + roffLine(line) + "\n", proseExample
	}
	return proseClose(st) + proseOpen(st, proseText) + roffLine(line) + "\n", proseText
}

// proseOpen emits the opening request for want when st is not already inside
// that block kind.
func proseOpen(st, want proseState) string {
	if st == want {
		return ""
	}
	if want == proseExample {
		return ".EX\n"
	}
	return ".PP\n"
}

// proseClose ends an open .EX block; filled paragraphs need no terminator.
func proseClose(st proseState) string {
	if st == proseExample {
		return ".EE\n"
	}
	return ""
}

// isIndented reports whether line belongs to a literal example block.
func isIndented(line manLine) bool {
	return strings.HasPrefix(string(line), " ") || strings.HasPrefix(string(line), "\t")
}

// roffLine escapes one line of plain text for roff: backslashes become the
// printable \e, and a leading control character (. or ') is neutralized with
// the \& zero-width guard.
func roffLine(line manLine) string {
	s := strings.ReplaceAll(string(line), `\`, `\e`)
	if strings.HasPrefix(strings.TrimLeft(s, " \t"), ".") || strings.HasPrefix(s, "'") {
		s = `\&` + s
	}
	return s
}

// docFlag pairs a flag's documentation surface with its names.
type docFlag struct {
	doc   cli.DocGenerationFlag
	names []string
}

// documentedFlags filters flags to the documentable ones, skipping the
// injected help flag.
func documentedFlags(flags []cli.Flag) []docFlag {
	out := make([]docFlag, 0, len(flags))
	for _, f := range flags {
		doc, ok := f.(cli.DocGenerationFlag)
		if !ok || f.Names()[0] == helpName {
			continue
		}
		out = append(out, docFlag{doc: doc, names: f.Names()})
	}
	return out
}

// manFlagsSection renders flags under an .SH heading, or nothing when no
// documentable flags remain.
func manFlagsSection(heading manHeading, flags []cli.Flag) string {
	docs := documentedFlags(flags)
	if len(docs) == 0 {
		return ""
	}
	return ".SH " + string(heading) + "\n" + manFlags(docs)
}

// manFlags renders each flag as a .TP tagged paragraph.
func manFlags(docs []docFlag) string {
	chunks := make([]string, len(docs))
	for i, f := range docs {
		chunks[i] = ".TP\n" + flagTag(f) + "\n" + roffLine(manLine(flagBody(f))) + "\n"
	}
	return strings.Join(chunks, "")
}

// flagTag renders the bold, dash-escaped flag names, with =value appended
// when the flag takes one.
func flagTag(f docFlag) string {
	parts := make([]string, len(f.names))
	for i, n := range f.names {
		dash := `\-\-`
		if len(n) == 1 {
			dash = `\-`
		}
		parts[i] = `\fB` + dash + strings.ReplaceAll(n, "-", `\-`) + `\fR`
	}
	tag := strings.Join(parts, ", ")
	if f.doc.TakesValue() {
		tag += `=\fIvalue\fR`
	}
	return tag
}

// flagBody is the flag's usage sentence plus its visible default, if any.
func flagBody(f docFlag) string {
	body := f.doc.GetUsage()
	if d := flagDefault(f.doc); d != "" {
		body += " (default: " + d + ")"
	}
	return body
}

// flagDefault mirrors urfave's help rendering of a flag default: the explicit
// DefaultText when set, else the flag's initial value when it takes one, and
// nothing when the default is hidden.
func flagDefault(doc cli.DocGenerationFlag) string {
	if !doc.IsDefaultVisible() {
		return ""
	}
	if d := doc.GetDefaultText(); d != "" {
		return d
	}
	if doc.TakesValue() {
		return doc.GetValue()
	}
	return ""
}

// manCommandsSection renders every visible subcommand as an .SS subsection
// under one COMMANDS section (VisibleCommands already excludes the injected
// help command).
func manCommandsSection(root *cli.Command) string {
	cmds := root.VisibleCommands()
	if len(cmds) == 0 {
		return ""
	}
	chunks := make([]string, len(cmds)+1)
	chunks[0] = ".SH COMMANDS\n"
	for i, cmd := range cmds {
		chunks[i+1] = manCommandEntry(binName(root.Name), cmd)
	}
	return strings.Join(chunks, "")
}

// manCommandEntry renders one subcommand: heading, synopsis line, usage
// sentence, long description, and flags.
func manCommandEntry(app binName, cmd *cli.Command) string {
	return fmt.Sprintf(".SS %q\n", strings.Join(cmd.Names(), ", ")) +
		commandSynopsis(app, cmd) +
		manProse(manText(cmd.Usage)) +
		manProse(manText(cmd.Description)) +
		manFlags(documentedFlags(cmd.VisibleFlags()))
}

// commandSynopsis renders the one-line invocation form for a subcommand.
func commandSynopsis(app binName, cmd *cli.Command) string {
	line := "[\\fIoptions\\fR]"
	if cmd.ArgsUsage != "" {
		line += " " + roffLine(manLine(cmd.ArgsUsage))
	}
	return ".B " + string(app) + " " + cmd.Name + "\n" + line + "\n"
}
