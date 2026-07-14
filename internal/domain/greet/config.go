package greet

// Config holds the flags and arguments for the greet command. Its fields are
// bound by the CLI tier and read by Run; it carries no behavior.
type Config struct {
	Greeting          salutation
	Repeat            repeatCount
	UppercaseEnabled  uppercaseEnabled
	EnthusiastEnabled enthusiastEnabled
}
