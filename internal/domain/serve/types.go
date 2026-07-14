package serve

// Named types for the host and port Config fields, bound by the CLI via pointer
// conversion. ShutdownTimeout stays a time.Duration since that already names the
// domain concept precisely.
type (
	host string // host is the bind address (--host).
	port int    // port is the listen port (--port).
)
