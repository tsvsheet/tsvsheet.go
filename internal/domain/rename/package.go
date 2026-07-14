// Package rename orchestrates the rename command.
//
// It defines the command's Config (the --dry-run flag the CLI binds) and Run (the
// orchestration entry point the CLI invokes). Run validates the optional name
// argument, then discovers the current and target identities, builds the rewrite
// plan, and applies it by calling the reusable gomatic/go-rewrite engine; it
// contains no CLI, flag, or output-formatting logic. This is the domain tier: the
// seam between the app tier (internal/app/commands/rename) and the implementation
// tier (gomatic/go-rewrite, gomatic/go-module).
package rename
