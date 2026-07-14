// Package greet orchestrates the greet command.
//
// It defines the command's Config (the flags and arguments the CLI binds) and
// Run (the orchestration entry point the CLI invokes). Run validates input and
// composes the greeting by calling the reusable internal/greeting package; it
// contains no CLI, flag, or output-formatting logic. This is the domain tier:
// the seam between the app tier (internal/app/commands/greet) and the
// implementation tier (internal/greeting).
package greet
