// Command tsv computes a .tsvt spreadsheet — a TAB-separated grid of
// literal and =formula cells — and edits it in the browser or terminal. The
// command tree lives in internal/cli.
package main

import (
	"context"
	"os"

	"github.com/tsvsheet/tsvsheet.go/internal/cli"
)

// version is the application version, set via ldflags: -X main.version=1.0.0.
var version = "dev"

// osExit is indirected so tests can observe the process exit code.
var osExit = os.Exit

func main() { osExit(run(os.Args)) }

// run wires the build version into the CLI and executes it, returning the
// process exit code. Keeping the logic here (rather than in main) makes the
// whole entry path testable.
func run(args []string) int {
	return cli.Run(context.Background(), cli.Version(version), args)
}
