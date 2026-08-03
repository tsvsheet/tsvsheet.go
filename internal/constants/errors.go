// Package constants declares tsvsheet's sentinel error values. The error
// mechanism (the matchable string type) lives in the shared gomatic/go-error
// library; these values are this package's own.
package constants

// The library's package is named errs; the explicit alias states that beside
// the import path (go-error), which does not name it.
import errs "github.com/gomatic/go-error"

// Keep these constants sorted alphabetically.
const (
	ErrApplyBothStdin      errs.Const = "sheet and edits cannot both be stdin"
	ErrDataListen          errs.Const = "failed to bind the data server"
	ErrDataRoot            errs.Const = "data directory cannot be published"
	ErrDiagnostics         errs.Const = "sheet has diagnostics"
	ErrForbidden           errs.Const = "cross-origin request refused"
	ErrImportContentType   errs.Const = "import content-type malformed"
	ErrImportEscape        errs.Const = "import reference escapes the data base"
	ErrImportFetch         errs.Const = "import fetch failed"
	ErrImportHostDenied    errs.Const = "import host not allowed"
	ErrImportNoBase        errs.Const = "import reference is relative but no data base is configured"
	ErrImportRead          errs.Const = "import body read failed"
	ErrImportRedirect      errs.Const = "import redirect refused"
	ErrImportScheme        errs.Const = "import scheme not permitted for host"
	ErrImportServeExposed  errs.Const = "imports refused: server bound to a non-loopback address"
	ErrImportStatus        errs.Const = "import response status not ok"
	ErrImportTooLarge      errs.Const = "import body too large"
	ErrImportURL           errs.Const = "import url invalid"
	ErrInvalidName         errs.Const = "invalid name"
	ErrManPage             errs.Const = "failed to render man page"
	ErrMissingArgument     errs.Const = "missing required argument"
	ErrMultiCellExpression errs.Const = "expression must be a single cell: it may not contain tab or newline characters"
	ErrOpenFile            errs.Const = "failed to open file"
	ErrOutsideGrid         errs.Const = "cell is outside the computed grid"
	ErrServeAddress        errs.Const = "malformed serve address"
	ErrServeExposed        errs.Const = "refusing a non-loopback bind"
	ErrSourceChanged       errs.Const = "file changed on disk — reopen"
	ErrUnknownFormat       errs.Const = "unknown render format"
	ErrUnknownHiddenPolicy errs.Const = "unknown hidden policy"
	ErrUnsupported         errs.Const = "unsupported construct"
	ErrUnsupportedShell    errs.Const = "unsupported shell"
	ErrUsage               errs.Const = "usage"
)
