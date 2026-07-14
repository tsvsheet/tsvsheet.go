package greet

// Named types for every Config field. The CLI binds flags to these via pointer
// conversion (e.g. (*string)(&cfg.Greeting)); naming the domain concept keeps
// the Config self-describing and avoids bare primitives.
type (
	salutation        string // salutation is the greeting word (--greeting).
	uppercaseEnabled  bool   // uppercaseEnabled toggles uppercase output (--uppercase).
	repeatCount       int    // repeatCount is how many times to repeat (--repeat).
	enthusiastEnabled bool   // enthusiastEnabled toggles extra emphasis (--enthusiast).
)
