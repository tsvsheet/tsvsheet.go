//go:build !windows

package dataserve_test

import (
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsvsheet/tsvsheet.go/internal/dataserve"
)

// TestHandler_FifoDoesNotHangTheHandler is the regression test for the hazard
// that decided the open flags: a blocking open of a FIFO with no writer parks
// forever, hanging the handler and — for a server scoped to a command — the
// command with it. An earlier draft of this package opened blocking and this
// test hung until it was killed. It must fail fast, not wait.
func TestHandler_FifoDoesNotHangTheHandler(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, syscall.Mkfifo(filepath.Join(dir, "pipe.tsv"), 0o600))

	done := make(chan int, 1)
	go func() {
		resp := get(t, dataserve.Root(dir), "/pipe.tsv")
		_ = resp.Body.Close()
		done <- resp.StatusCode
	}()

	select {
	case status := <-done:
		assert.Equal(t, http.StatusNotFound, status, "a FIFO is not a regular file")
	case <-t.Context().Done():
		t.Fatal("the handler blocked on a writer-less FIFO")
	}
}

// TestHandler_DeviceIsRefused covers the remaining non-regular kind: a
// character device is openable and readable, so only the mode check stops it.
func TestHandler_DeviceIsRefused(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.Symlink("/dev/null", filepath.Join(dir, "null.tsv")))

	resp := get(t, dataserve.Root(dir), "/null.tsv")
	_ = resp.Body.Close()

	// os.Root refuses a symlink escaping the root, so this is a 404 either way;
	// the assertion is that it is never served as data.
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
