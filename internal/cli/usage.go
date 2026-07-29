package cli

import (
	"errors"

	"github.com/urfave/cli/v3"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

// missingArgument reports an omitted required positional by printing the
// command's own help, then returning constants.ErrUsage so the process exits
// non-zero without logging.
//
// A structured log line is the wrong answer to a usage mistake. "missing
// required argument: argument dir" tells a reader what they already know — that
// something is missing — and nothing about what to type instead; the help says
// exactly that, including the argument's form and a worked example. A usage
// mistake is also not a runtime failure, so it has no business in the log
// stream a real failure shares.
func missingArgument(c *cli.Command) error {
	_ = cli.ShowSubcommandHelp(c)
	return constants.ErrUsage
}

// withUsageHelp turns a missing-argument failure raised anywhere inside a
// command's run into that command's help. Some required values are only
// discovered deep in the run — eval's expression may come from stdin, so it is
// not known to be absent until stdin has been read — and the answer is the same
// wherever it surfaces.
func withUsageHelp(c *cli.Command, err error) error {
	if errors.Is(err, constants.ErrMissingArgument) {
		return missingArgument(c)
	}
	return err
}

// isUsage reports whether an error is a usage mistake whose help has already
// been printed, so the top level exits quietly rather than logging it.
func isUsage(err error) bool { return errors.Is(err, constants.ErrUsage) }
