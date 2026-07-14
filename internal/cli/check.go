package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/uplang/tsvsheet.go/internal/constants"
	"github.com/uplang/tsvsheet.go/internal/sheet"
)

// checkConfig binds the check command's template source path.
type checkConfig struct {
	template sourcePath
}

// runCheck validates a template, writing one diagnostic per line to the error
// stream. It returns ErrSyntax on a parse failure (exit 2), ErrDiagnostics when
// the template parses but has findings (exit 1), or nil when clean (exit 0).
func runCheck(streams Streams, cfg checkConfig) error {
	reader, release, err := cfg.template.open(streams.In)
	if err != nil {
		return err
	}
	defer func() { _ = release() }()

	tmpl, err := parseTemplate(reader)
	if err != nil {
		return err
	}
	return reportDiagnostics(streams.Err, sheet.Check(tmpl))
}

// reportDiagnostics writes each diagnostic to w and returns ErrDiagnostics when
// any are present.
func reportDiagnostics(w io.Writer, diags []sheet.Diagnostic) error {
	for _, d := range diags {
		fmt.Fprintf(w, "line %d: %s\n", d.Line, d.Message)
	}
	if len(diags) > 0 {
		return constants.ErrDiagnostics.With(nil, "count", len(diags))
	}
	return nil
}

// isSyntaxError reports whether err is a template syntax error (for exit-code
// mapping).
func isSyntaxError(err error) bool { return errors.Is(err, constants.ErrSyntax) }

// isDiagnostics reports whether err signals that check found diagnostics.
func isDiagnostics(err error) bool { return errors.Is(err, constants.ErrDiagnostics) }

// checkCommand builds the `check` command.
func checkCommand() *cli.Command {
	cfg := checkConfig{}
	return &cli.Command{
		Name:      "check",
		Usage:     "Validate a template and report diagnostics.",
		ArgsUsage: " ",
		Description: `Parse and statically check a .tsvt template. Diagnostics are written one per
line to stderr. Exit status: 0 clean, 1 diagnostics found, 2 syntax error.

Examples:
  tsvsheet check --template sheet.tsvt
  cat sheet.tsvt | tsvsheet check`,
		Flags:  templateFlagOnly(&cfg.template),
		Action: streamAction(func(s Streams) error { return runCheck(s, cfg) }),
	}
}
