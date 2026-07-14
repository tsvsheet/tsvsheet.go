// Package set orchestrates the "config set" command.
//
// Run validates the key/value pair and either writes it to the store or, in
// dry-run mode, reports what would change without writing. It contains no CLI
// or output-formatting logic. This is the domain tier between the app tier
// (internal/app/commands/config/set) and the implementation tier
// (internal/config).
package set
