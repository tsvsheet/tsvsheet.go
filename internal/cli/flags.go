package cli

import (
	"io"
	"os"
)

// stdin is indirected so tests can substitute an input stream.
var stdin io.Reader = os.Stdin

// stderr is indirected so tests can capture diagnostics.
var stderr io.Writer = os.Stderr

// jsonFlag names the explain command's --json option.
const jsonFlag = "json"

// argsUsageSheet is the positional form every command that computes a sheet
// declares, so the three that spell it cannot drift apart in help output.
const argsUsageSheet = "<sheet>"

// positional is a command's positional arguments. Required inputs (the .tsvt
// spreadsheet path, and the cell address for explain) are positional — never
// flags — so invocations read as `tsv explain D2 sheet.tsvt`.
type positional []string

// at returns the i-th positional argument as a source path, or "" (meaning
// stdin) when the argument is absent.
func (p positional) at(i int) sourcePath {
	if i < len(p) {
		return sourcePath(p[i])
	}
	return ""
}

// text returns the i-th positional argument verbatim, or "" when absent.
func (p positional) text(i int) string {
	if i < len(p) {
		return p[i]
	}
	return ""
}
