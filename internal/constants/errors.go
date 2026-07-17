// Package constants declares tsvsheet's sentinel error values. The error
// mechanism (the matchable string type) lives in the shared gomatic/go-error
// library; these values are this package's own.
package constants

// Imported bare (the package is named error); this file declares only sentinels
// and uses no builtin error type, so each declaration reads errs.Const.
import errs "github.com/gomatic/go-error"

// Keep these constants sorted alphabetically.
const (
	ErrDiagnostics        errs.Const = "sheet has diagnostics"
	ErrForbidden          errs.Const = "cross-origin request refused"
	ErrImportContentType  errs.Const = "import content-type malformed"
	ErrImportFetch        errs.Const = "import fetch failed"
	ErrImportHostDenied   errs.Const = "import host not allowed"
	ErrImportRead         errs.Const = "import body read failed"
	ErrImportRedirect     errs.Const = "import redirect refused"
	ErrImportScheme       errs.Const = "import scheme not permitted for host"
	ErrImportServeExposed errs.Const = "imports refused: server bound to a non-loopback address"
	ErrImportStatus       errs.Const = "import response status not ok"
	ErrImportTooLarge     errs.Const = "import body too large"
	ErrImportURL          errs.Const = "import url invalid"
	ErrInvalidName        errs.Const = "invalid name"
	ErrMissingArgument    errs.Const = "missing required argument"
	ErrOpenFile           errs.Const = "failed to open file"
	ErrUnknownFormat      errs.Const = "unknown render format"
	ErrUnsupported        errs.Const = "unsupported construct"
	ErrUnsupportedShell   errs.Const = "unsupported shell"
)
