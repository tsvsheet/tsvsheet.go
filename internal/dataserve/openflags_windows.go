//go:build windows

package dataserve

import "os"

// openFlags opens a published file read-only. Windows has no O_NONBLOCK and no
// FIFO in the POSIX sense, so the unix non-blocking guard has nothing to guard
// against here; writeFile still refuses anything that is not a regular file.
const openFlags = os.O_RDONLY
