// Package constants declares template.cli's sentinel error values. The error
// mechanism (the matchable string type) lives in the shared gomatic/go-error
// library; these values are this template's own.
package constants

// Imported bare (the package is named error); this file declares only sentinels
// and uses no builtin error type, so each declaration reads errs.Const.
import errs "github.com/gomatic/go-error"

// Keep these constants sorted alphabetically.
const (
	ErrGitCommand      errs.Const = "git command failed"
	ErrInvalidName     errs.Const = "invalid name"
	ErrInvalidValue    errs.Const = "invalid value"
	ErrMissingArgument errs.Const = "missing required argument"
	ErrNotFound        errs.Const = "not found"
	ErrOpenFile        errs.Const = "failed to open file"
	ErrReadInput       errs.Const = "failed to read input"
	ErrWriteFile       errs.Const = "failed to write file"
)
