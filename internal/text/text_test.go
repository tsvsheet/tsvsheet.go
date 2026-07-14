package text

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gomatic/template.cli/internal/constants"
)

func TestPrimitives(t *testing.T) {
	t.Parallel()
	want := assert.New(t)
	want.Equal(Line("HELLO"), Uppercase("hello"))
	want.Equal(Line(">> line"), WithPrefix("line", ">> "))
	want.Equal(Line("   1 | line"), Numbered("line", 1))
	want.True(Contains("keep this", "keep"))
	want.False(Contains("drop this", "keep"))
}

// keepAll keeps every line unchanged.
func keepAll(line Line, _ LineNumber) (Line, bool) { return line, true }

func TestProcess(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		input     string
		transform Transform
		want      Output
	}{
		{
			name:      "passthrough",
			input:     "a\nb\nc",
			transform: keepAll,
			want:      "a\nb\nc",
		},
		{
			name:  "filter drops non-matching",
			input: "keep\ndrop\nkeep me",
			transform: func(line Line, _ LineNumber) (Line, bool) {
				return line, Contains(line, "keep")
			},
			want: "keep\nkeep me",
		},
		{
			name:  "numbered uppercase",
			input: "hi\nyo",
			transform: func(line Line, number LineNumber) (Line, bool) {
				return Numbered(Uppercase(line), number), true
			},
			want: "   1 | HI\n   2 | YO",
		},
		{
			name:      "empty input",
			input:     "",
			transform: keepAll,
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, must := assert.New(t), require.New(t)

			output, err := Process(context.Background(), strings.NewReader(tt.input), tt.transform)
			must.NoError(err)
			want.Equal(tt.want, output)
		})
	}
}

func TestProcess_ContextCancelled(t *testing.T) {
	t.Parallel()
	want, must := assert.New(t), require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Process(ctx, strings.NewReader("a\nb\nc"), keepAll)
	must.Error(err)
	want.ErrorIs(err, context.Canceled)
}

// failingReader returns an error partway through to exercise scanner failures.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, constants.ErrReadInput
}

func TestProcess_ReadError(t *testing.T) {
	t.Parallel()
	want, must := assert.New(t), require.New(t)

	_, err := Process(context.Background(), failingReader{}, keepAll)
	must.Error(err)
	want.ErrorIs(err, constants.ErrReadInput)
}
