// Package list orchestrates the "config list" command.
//
// Run reads the configuration entries matching the optional prefix from the
// store and returns them. It contains no CLI or output-formatting logic. This
// is the domain tier between the app tier (internal/app/commands/config/list)
// and the implementation tier (internal/config).
package list
