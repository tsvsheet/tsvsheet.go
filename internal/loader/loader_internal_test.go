// White-box failure-leg tests for the bounded sibling load: the sheetFile
// seam makes every branch of parse reachable.
package loader

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tsvsheet "github.com/tsvsheet/go-tsvsheet"
)

// faultSheet drives parse's failure branches through the sheetFile seam: each
// leg (stat, census scan, buffering) can be made to fail independently.
type faultSheet struct {
	data      *bytes.Reader
	statErr   error
	readAtErr error
	readErr   error
}

func (f faultSheet) Read(p []byte) (int, error) {
	if f.readErr != nil {
		return 0, f.readErr
	}
	return f.data.Read(p)
}

func (f faultSheet) ReadAt(p []byte, off int64) (int, error) {
	if f.readAtErr != nil {
		return 0, f.readAtErr
	}
	return f.data.ReadAt(p, off)
}

func (f faultSheet) Stat() (os.FileInfo, error) {
	if f.statErr != nil {
		return nil, f.statErr
	}
	return fileInfo{size: f.data.Size()}, nil
}

// fileInfo carries only the size parse consults.
type fileInfo struct{ size int64 }

func (f fileInfo) Name() string       { return "fault.tsvt" }
func (f fileInfo) Size() int64        { return f.size }
func (f fileInfo) Mode() os.FileMode  { return 0 }
func (f fileInfo) ModTime() time.Time { return time.Time{} }
func (f fileInfo) IsDir() bool        { return false }
func (f fileInfo) Sys() any           { return nil }

// TestParse_EachFailureLegSurfaces pins every failure branch of the bounded
// sibling load: a failed stat, a failed census scan, and a failed buffering
// read each surface their cause — never a silent empty sheet.
func TestParse_EachFailureLegSurfaces(t *testing.T) {
	t.Parallel()

	data := func() *bytes.Reader { return bytes.NewReader([]byte("a\tb\n")) }
	limits := tsvsheet.DefaultLimits()

	_, _, err := parse(faultSheet{data: data(), statErr: assert.AnError}, "t", limits)
	require.ErrorIs(t, err, assert.AnError, "stat failure surfaces")

	_, _, err = parse(faultSheet{data: data(), readAtErr: assert.AnError}, "t", limits)
	require.ErrorIs(t, err, tsvsheet.ErrReadInput, "a census scan failure is a read failure")

	_, _, err = parse(faultSheet{data: data(), readErr: assert.AnError}, "t", limits)
	require.ErrorIs(t, err, assert.AnError, "a buffering failure surfaces")

	sheet, path, err := parse(faultSheet{data: data()}, "ok.tsvt", limits)
	require.NoError(t, err)
	assert.Equal(t, tsvsheet.Path("ok.tsvt"), path)
	assert.Equal(t, tsvsheet.Grid{{"a", "b"}}, sheet.Source())
}
