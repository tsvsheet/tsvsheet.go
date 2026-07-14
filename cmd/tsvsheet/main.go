// Command tsvsheet computes .tsvt templates over .tsv data and edits worksheets
// in the browser or terminal. The command tree lives in internal/cli.
package main

import (
	"context"
	"os"

	"github.com/uplang/tsvsheet.go/internal/cli"
)

// version is the application version, set via ldflags: -X main.version=1.0.0.
var version = "dev"

// osExit is indirected so tests can observe the process exit code.
var osExit = os.Exit

func main() {
	cli.Version = version
	osExit(cli.Run(context.Background(), os.Args))
}
