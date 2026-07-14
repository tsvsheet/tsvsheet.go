package process

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	app "github.com/gomatic/go-app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"github.com/gomatic/template.cli/internal/constants"
)

func TestProcessCommand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		wantErr      error
		name         string
		stdin        string
		inputFile    string
		wantContains string
		args         []string
	}{
		{
			name:         "stdin passthrough",
			args:         []string{"app", "process"},
			stdin:        "line 1\nline 2",
			wantContains: `"output": "line 1\nline 2"`,
		},
		{
			name:         "stdin uppercase",
			args:         []string{"app", "process", "--uppercase"},
			stdin:        "hello world",
			wantContains: `"output": "HELLO WORLD"`,
		},
		{
			name:         "stdin with prefix",
			args:         []string{"app", "process", "--prefix", ">> "},
			stdin:        "test",
			wantContains: `>> test`,
		},
		{
			name:         "stdin with filter",
			args:         []string{"app", "process", "--filter=keep"},
			stdin:        "keep this\nremove this\nkeep that",
			wantContains: `keep this\nkeep that`,
		},
		{
			name:         "file input",
			inputFile:    "line 1\nline 2\nline 3",
			wantContains: `line 1\nline 2\nline 3`,
		},
		{
			name:    "file not found",
			args:    []string{"app", "process", "/nonexistent/file.txt"},
			wantErr: constants.ErrOpenFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, must := assert.New(t), require.New(t)

			var stdout bytes.Buffer
			var stderr bytes.Buffer

			logger := slog.New(slog.NewTextHandler(&stderr, &slog.HandlerOptions{
				Level: slog.LevelWarn,
			}))

			// Setup args - either with or without file
			args := tt.args
			if tt.inputFile != "" {
				// Create temporary file
				tmpDir := t.TempDir()
				tmpFile := filepath.Join(tmpDir, "test.txt")
				err := os.WriteFile(tmpFile, []byte(tt.inputFile), 0o644)
				must.NoError(err, "failed to create temp file")
				args = []string{"app", "process", tmpFile}
			} else if tt.args == nil {
				args = []string{"app", "process"}
			}

			// Setup stdin if needed
			if tt.stdin != "" {
				oldStdin := os.Stdin
				stdinR, stdinW, _ := os.Pipe()
				os.Stdin = stdinR
				t.Cleanup(func() { os.Stdin = oldStdin })

				// Write test data to stdin
				go func() {
					_, _ = stdinW.Write([]byte(tt.stdin))
					_ = stdinW.Close()
				}()
			}

			app := &cli.Command{
				Name:      "app",
				Writer:    &stdout,
				ErrWriter: &stderr,
				Commands: []*cli.Command{
					Command(),
				},
				Metadata: map[string]any{
					app.LoggerMetadataKey: logger,
				},
			}

			err := app.Run(context.Background(), args)

			if tt.wantErr != nil {
				must.Error(err)
				want.ErrorIs(err, tt.wantErr)
				want.ErrorContains(err, tt.wantErr.Error())
				return
			}

			must.NoError(err)

			// Verify JSON output contains expected data
			output := stdout.String()
			want.Contains(output, tt.wantContains)
		})
	}
}
