package list

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	app "github.com/gomatic/go-app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func TestListCommand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		args         []string
		wantErr      error
		wantContains []string
	}{
		{
			name: "list all",
			args: []string{"list"},
			wantContains: []string{
				`"app.name": "template.cli"`,
				`"database.host": "localhost"`,
			},
		},
		{
			name: "list with prefix",
			args: []string{"list", "--prefix", "app."},
			wantContains: []string{
				`"app.name": "template.cli"`,
				`"app.version": "1.0.0"`,
			},
		},
		{
			name: "list with non-matching prefix",
			args: []string{"list", "--prefix", "nonexistent."},
			wantContains: []string{
				`{}`,
			},
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

			// Verify JSON output contains expected keys
			output := buf.String()
			for _, expected := range tt.wantContains {
				want.Contains(output, expected)
			}
		})
	}
}
