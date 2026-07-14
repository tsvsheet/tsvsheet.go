package greet

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"testing"

	app "github.com/gomatic/go-app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"github.com/gomatic/template.cli/internal/constants"
)

func TestGreetCommand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		wantErr      error
		envVars      map[string]string
		name         string
		wantContains string
		args         []string
	}{
		{
			name:         "simple greeting",
			args:         []string{"app", "greet", "World"},
			wantContains: `"message": "Hello, World!"`,
		},
		{
			name:         "custom greeting",
			args:         []string{"app", "greet", "--greeting=Hi", "Alice"},
			wantContains: `"message": "Hi, Alice!"`,
		},
		{
			name:         "uppercase",
			args:         []string{"app", "greet", "--uppercase", "Bob"},
			wantContains: `"message": "HELLO, BOB!"`,
		},
		{
			name:         "repeat",
			args:         []string{"app", "greet", "--repeat=2", "Charlie"},
			wantContains: `Hello, Charlie!\nHello, Charlie!`,
		},
		{
			name:         "enthusiast mode",
			args:         []string{"app", "greet", "--enthusiast", "Dave"},
			wantContains: `"message": "Hello, Dave!!!"`,
		},
		{
			name:    "missing name",
			args:    []string{"app", "greet"},
			wantErr: constants.ErrMissingArgument,
		},
		{
			name:         "environment variable",
			args:         []string{"app", "greet", "Partner"},
			envVars:      map[string]string{"GREETING": "Howdy"},
			wantContains: `"message": "Howdy, Partner!"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, must := assert.New(t), require.New(t)

			// Set environment variables if specified
			for key, value := range tt.envVars {
				os.Setenv(key, value)
				defer os.Unsetenv(key)
			}

			var stdout bytes.Buffer
			var stderr bytes.Buffer

			logger := slog.New(slog.NewTextHandler(&stderr, &slog.HandlerOptions{
				Level: slog.LevelWarn,
			}))

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

			err := app.Run(context.Background(), tt.args)

			if tt.wantErr != nil {
				must.Error(err)
				want.ErrorIs(err, tt.wantErr)
				want.ErrorContains(err, tt.wantErr.Error())
				return
			}

			must.NoError(err)

			// Verify JSON output contains expected message
			output := stdout.String()
			want.Contains(output, tt.wantContains)
		})
	}
}
