package process

// Config holds the flags and argument for the process command.
type Config struct {
	FilePath           filePath
	Prefix             prefix
	Filter             filter
	UppercaseEnabled   uppercaseEnabled
	LineNumbersEnabled lineNumbersEnabled
}
