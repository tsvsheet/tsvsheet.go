// Package get orchestrates the "config get" command.
//
// Run validates the requested key, reads it from the configuration store, and
// applies the configured default when the key is absent. It contains no CLI or
// output-formatting logic. This is the domain tier between the app tier
// (internal/app/commands/config/get) and the implementation tier
// (internal/config).
package get
