package sheet_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/constants"
	"github.com/uplang/tsvsheet.go/internal/sheet"
)

func TestReadTSV(t *testing.T) {
	t.Parallel()

	g, err := sheet.ReadTSV(strings.NewReader("a\tb\n1\t2\n"))
	require.NoError(t, err)
	assert.Equal(t, sheet.Grid{{"a", "b"}, {"1", "2"}}, g)
}

func TestReadTSV_Ragged(t *testing.T) {
	t.Parallel()

	g, err := sheet.ReadTSV(strings.NewReader("a\tb\tc\n1\n"))
	require.NoError(t, err)
	assert.Equal(t, sheet.Grid{{"a", "b", "c"}, {"1"}}, g)
}

func TestReadTSV_Empty(t *testing.T) {
	t.Parallel()

	g, err := sheet.ReadTSV(strings.NewReader(""))
	require.NoError(t, err)
	assert.Empty(t, g)
}

// failingReader always errors, exercising the ReadTSV scan-error path.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errReadTest }

var errReadTest = errors.New("read failed")

func TestReadTSV_Error(t *testing.T) {
	t.Parallel()

	_, err := sheet.ReadTSV(failingReader{})
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrReadInput)
}

func TestWriteTSV(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	require.NoError(t, sheet.WriteTSV(&b, sheet.Grid{{"a", "b"}, {"1", "2"}}))
	assert.Equal(t, "a\tb\n1\t2\n", b.String())
}

// failingWriter errors after n successful bytes, exercising the WriteTSV error
// path.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWriteTest }

var errWriteTest = errors.New("write failed")

func TestWriteTSV_Error(t *testing.T) {
	t.Parallel()

	err := sheet.WriteTSV(failingWriter{}, sheet.Grid{{"a"}})
	require.Error(t, err)
	assert.ErrorIs(t, err, constants.ErrWriteFile)
}

func TestReadWriteRoundTrip(t *testing.T) {
	t.Parallel()

	const in = "1\t2\t3\n4\t5\t6\n"
	g, err := sheet.ReadTSV(strings.NewReader(in))
	require.NoError(t, err)

	var b strings.Builder
	require.NoError(t, sheet.WriteTSV(&b, g))
	assert.Equal(t, in, b.String())
}
