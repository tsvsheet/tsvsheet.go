package process

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gomatic/template.cli/internal/constants"
	"github.com/gomatic/template.cli/internal/text"
)

func TestRun(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		input  string
		want   text.Output
		config Config
	}{
		{name: "passthrough", input: "line 1\nline 2", config: Config{}, want: "line 1\nline 2"},
		{name: "empty input", input: "", config: Config{}, want: ""},
		{name: "uppercase", input: "hello\nworld", config: Config{UppercaseEnabled: true}, want: "HELLO\nWORLD"},
		{name: "line numbers", input: "a\nb", config: Config{LineNumbersEnabled: true}, want: "   1 | a\n   2 | b"},
		{name: "prefix", input: "x\ny", config: Config{Prefix: ">> "}, want: ">> x\n>> y"},
		{name: "filter", input: "keep\ndrop\nkeep me", config: Config{Filter: "keep"}, want: "keep\nkeep me"},
		{name: "filter no matches", input: "a\nb", config: Config{Filter: "zz"}, want: ""},
		{
			name:   "combined",
			input:  "hello\nworld",
			config: Config{UppercaseEnabled: true, Prefix: "=> ", LineNumbersEnabled: true},
			want:   "   1 | => HELLO\n   2 | => WORLD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, must := assert.New(t), require.New(t)

			dir := t.TempDir()
			path := filepath.Join(dir, "in.txt")
			must.NoError(os.WriteFile(path, []byte(tt.input), 0o600))

			logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
			result, err := Run(context.Background(), logger, tt.config, path)

			must.NoError(err)
			want.Equal(tt.want, result.Output)
		})
	}
}

func TestRun_FileNotFound(t *testing.T) {
	t.Parallel()
	want, must := assert.New(t), require.New(t)

	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	_, err := Run(context.Background(), logger, Config{}, "/no/such/file.txt")

	must.Error(err)
	want.ErrorIs(err, constants.ErrOpenFile)
}

func TestRun_ContextCancelled(t *testing.T) {
	t.Parallel()
	want, must := assert.New(t), require.New(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "in.txt")
	must.NoError(os.WriteFile(path, bytes.Repeat([]byte("line\n"), 1000), 0o600))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	_, err := Run(ctx, logger, Config{}, path)

	must.Error(err)
	want.ErrorIs(err, context.Canceled)
}

// TestRun_ConfigFilePath covers resolving the input from Config.FilePath when
// no positional argument is supplied.
func TestRun_ConfigFilePath(t *testing.T) {
	t.Parallel()
	want, must := assert.New(t), require.New(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "in.txt")
	must.NoError(os.WriteFile(path, []byte("only line"), 0o600))

	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	result, err := Run(context.Background(), logger, Config{FilePath: filePath(path)})

	must.NoError(err)
	want.Equal(text.Output("only line"), result.Output)
}

// TestOpenStdin covers the stdin branch of open without reading from the real
// stdin: it asserts the helper resolves to os.Stdin and a no-op closer.
func TestOpenStdin(t *testing.T) {
	t.Parallel()
	want, must := assert.New(t), require.New(t)

	reader, release, err := open("")
	must.NoError(err)
	want.Same(os.Stdin, reader)
	want.NoError(release())
}
