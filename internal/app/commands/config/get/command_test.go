package get

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	app "github.com/gomatic/go-app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"github.com/gomatic/template.cli/internal/constants"
)

func TestGetCommand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		wantErr      error
		name         string
		wantContains string
		args         []string
	}{
		{
			name:         "get existing key",
			args:         []string{"get", "app.name"},
			wantContains: `"value": "template.cli"`,
		},
		{
			name:         "get with default",
			args:         []string{"get", "--default", "fallback", "missing.key"},
			wantContains: `"value": "fallback"`,
		},
		{
			name:    "missing key without default",
			args:    []string{"get", "nonexistent.key"},
			wantErr: constants.ErrNotFound,
		},
		{
			name:    "missing argument",
			args:    []string{"get"},
			wantErr: constants.ErrMissingArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, must := assert.New(t), require.New(t)

			// Setup app with output capture
			var buf bytes.Buffer
			testApp := &cli.Command{
				Name:     "test",
				Commands: []*cli.Command{Command()},
				Writer:   &buf,
			}

			// Setup logger in metadata
			logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn}))
			testApp.Metadata = map[string]any{
				app.LoggerMetadataKey: logger,
			}

			// Execute command
			err := testApp.Run(context.Background(), append([]string{"test"}, tt.args...))

			// Verify error handling
			if tt.wantErr != nil {
				must.Error(err)
				want.ErrorIs(err, tt.wantErr)
				want.ErrorContains(err, tt.wantErr.Error())
				return
			}

			must.NoError(err)

			// Verify JSON output contains expected value
			if tt.wantContains != "" {
				want.Contains(buf.String(), tt.wantContains)
			}
		})
	}
}
