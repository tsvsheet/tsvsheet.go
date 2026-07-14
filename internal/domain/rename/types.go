package rename

// dryRunEnabled toggles dry-run mode (--dry-run); the CLI binds it by pointer. In
// dry-run mode Run reports what would change without writing or moving anything.
type dryRunEnabled bool
