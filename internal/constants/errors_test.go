package constants

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The error mechanism (Error/With) is exercised in gomatic/go-error; this test
// only verifies that this package's sentinels carry their text and remain
// matchable with errors.Is once wrapped — the contract consumers rely on.
func TestSentinels(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	want.Equal("unsupported construct", ErrUnsupported.Error())
	want.Equal("invalid name", ErrInvalidName.Error())

	wrapped := fmt.Errorf("%w: %s", ErrOpenFile, "config.json")
	want.ErrorIs(wrapped, ErrOpenFile)
	want.NotErrorIs(wrapped, ErrUnsupported)
}
