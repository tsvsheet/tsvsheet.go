package serve

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	app "github.com/gomatic/go-app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"github.com/gomatic/template.cli/internal/constants"
)

func TestServeCommand(t *testing.T) {
	tests := []struct {
		wantErr error
		name    string
		args    []string
		timeout time.Duration
	}{
		{
			name:    "invalid port - too high",
			args:    []string{"serve", "--port", "70000"},
			wantErr: constants.ErrInvalidValue,
			timeout: 1 * time.Second,
		},
		{
			name:    "invalid shutdown timeout",
			args:    []string{"serve", "--shutdown-timeout", "100ms"},
			wantErr: constants.ErrInvalidValue,
			timeout: 1 * time.Second,
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

			// Create a context that will be cancelled
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			// Execute command
			err := testApp.Run(ctx, append([]string{"test"}, tt.args...))

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

func TestServeCommand_GracefulShutdown(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "graceful shutdown completes successfully",
			args: []string{"test", "serve", "--port", "0", "--shutdown-timeout", "2s"},
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
			logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelInfo}))
			testApp.Metadata = map[string]any{
				app.LoggerMetadataKey: logger,
			}

			// Create a context that will be cancelled after a short time
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Run server in background
			serverDone := make(chan error, 1)
			go func() {
				serverDone <- testApp.Run(ctx, tt.args)
			}()

			// Give server time to start (using real time since HTTP servers don't work with synctest)
			time.Sleep(200 * time.Millisecond)

			// Cancel context to trigger shutdown
			cancel()

			// Wait for graceful shutdown
			select {
			case err := <-serverDone:
				must.NoError(err)
			case <-time.After(5 * time.Second):
				t.Fatal("server shutdown timed out")
			}

			// Verify no error was returned
			want.True(true) // Test passed if we got here
		})
	}
}
