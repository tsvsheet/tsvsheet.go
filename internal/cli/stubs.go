package cli

import "github.com/uplang/tsvsheet.go/internal/constants"

// runServe is implemented in serve_run.go (task 7); this placeholder keeps the
// command tree compiling until then.
func runServe(_ Streams, _ serveConfig) error {
	return constants.ErrUnsupported.With(nil, "message", "serve not yet implemented")
}

// runTUI is implemented in tui_run.go (task 8); placeholder until then.
func runTUI(_ Streams, _ tuiConfig) error {
	return constants.ErrUnsupported.With(nil, "message", "tui not yet implemented")
}
