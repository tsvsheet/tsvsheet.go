// The pager's source-drift guard (spec 017): a file mutated in place while
// the TUI pages it must refuse honestly, not render a mix of stale cache and
// live reads under a stale census.
package cli

import (
	"os"
	"time"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

// byteSize is a file length in bytes, as stat reports it.
type byteSize int64

// statSource reports the open file's size and modification time. It is a
// package variable so tests can force the stat failure branch — the
// sanctioned stdlib-fault seam.
var statSource = func(f *os.File) (byteSize, time.Time, error) {
	info, err := f.Stat()
	if err != nil {
		return 0, time.Time{}, err
	}
	return byteSize(info.Size()), info.ModTime(), nil
}

// sourceDrift builds the pager's per-window guard: it re-stats the HELD
// descriptor and refuses with ErrSourceChanged when the inode's size or mtime
// has drifted from what was opened (an in-place truncate, append, or
// rewrite). Detection is bounded by the filesystem's stat resolution: a
// same-length rewrite landing inside one mtime granule is invisible to stat —
// nanoseconds on APFS/ext4, coarser on FAT-class filesystems.
// A rename-replace — the ordinary safe save — swaps the path to a
// NEW inode, so the held descriptor sees no drift and paging continues on the
// opened snapshot, the same consistency the pre-016 editor's read-once load
// gave; TestDriftGuard_RenameReplaceKeepsTheSnapshot states that half, and
// TestDriftGuard_RefusesTruncateAndRewrite the refusing half.
func sourceDrift(f *os.File, size byteSize, mtime time.Time) func() error {
	return func() error {
		nowSize, nowMtime, err := statSource(f)
		if err != nil {
			return constants.ErrSourceChanged.With(err)
		}
		if nowSize != size {
			return constants.ErrSourceChanged.With(nil, "size", nowSize, "was", size)
		}
		if !nowMtime.Equal(mtime) {
			return constants.ErrSourceChanged.With(
				nil,
				"modified",
				nowMtime.Format(time.RFC3339Nano),
				"was",
				mtime.Format(time.RFC3339Nano),
			)
		}
		return nil
	}
}
