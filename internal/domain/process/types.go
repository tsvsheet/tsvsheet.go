package process

// Named types for every Config field, bound by the CLI via pointer conversion.
type (
	filePath           string // filePath is the optional input file (positional arg).
	uppercaseEnabled   bool   // uppercaseEnabled toggles uppercase output (--uppercase).
	lineNumbersEnabled bool   // lineNumbersEnabled toggles line numbering (--line-numbers).
	prefix             string // prefix is prepended to each kept line (--prefix).
	filter             string // filter keeps only lines containing it (--filter).
)
