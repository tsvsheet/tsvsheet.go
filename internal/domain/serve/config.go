package serve

import "time"

// Config holds the flags for the serve command.
type Config struct {
	Host            host
	Port            port
	ShutdownTimeout time.Duration
}
