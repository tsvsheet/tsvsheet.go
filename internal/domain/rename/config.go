package rename

// Config holds the flags for the rename command. Its fields are bound by the CLI
// tier and read by Run; it carries no behavior.
type Config struct {
	DryRunEnabled dryRunEnabled
}
