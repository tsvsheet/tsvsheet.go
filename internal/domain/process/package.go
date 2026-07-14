// Package process orchestrates the process command.
//
// Run resolves the input source (a file or stdin), builds a transform from the
// command's flags, and delegates line processing to the reusable internal/text
// package. It contains no CLI or output-formatting logic. This is the domain
// tier between the app tier (internal/app/commands/process) and the
// implementation tier (internal/text).
package process
