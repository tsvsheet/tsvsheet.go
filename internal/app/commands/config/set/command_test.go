package set

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

func TestSetCommand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		wantErr error
		name    string
		args    []string
	}{
		{
			name: "set value",
			args: []string{"set", "test.key", "test-value"},
		},
		{
			name: "set with dry-run",
			args: []string{"set", "test.key", "test-value", "--dry-run"},
		},
		{
			name: "set with short dry-run flag",
			args: []string{"set", "test.key", "test-value", "-n"},
		},
		{
			name:    "missing arguments",
			args:    []string{"set"},
			wantErr: constants.ErrMissingArgument,
		},
		{
			name:    "missing value",
			args:    []string{"set", "key.only"},
			wantErr: constants.ErrMissingArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, must := assert.New(t), require.New(t)

			// Setup app
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
		})
	}
}
