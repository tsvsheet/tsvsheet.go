// Package serve orchestrates the serve command.
//
// Run validates the server configuration and delegates the listening and
// graceful-shutdown lifecycle to the reusable gomatic/go-httpserver package. It
// contains no CLI or output-formatting logic. This is the domain tier between
// the app tier (internal/app/commands/serve) and the implementation tier
// (gomatic/go-httpserver).
package serve
