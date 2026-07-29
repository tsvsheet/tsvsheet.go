//go:build !windows

package dataserve

import (
	"os"
	"syscall"
)

// openFlags opens a published file read-only and NON-BLOCKING. The non-blocking
// bit is the whole point: a blocking open of a FIFO parks until a writer
// appears, which would hang the handler and, for a server scoped to a command,
// the command itself. With O_NONBLOCK the open returns immediately and
// writeFile refuses the file on its mode.
const openFlags = os.O_RDONLY | syscall.O_NONBLOCK
