package serve

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gomatic/template.cli/internal/constants"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

func TestRun_InvalidConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		wantErr error
		name    string
		config  Config
	}{
		{
			name:    "negative port",
			config:  Config{Host: "127.0.0.1", Port: -1, ShutdownTimeout: 5 * time.Second},
			wantErr: constants.ErrInvalidValue,
		},
		{
			name:    "port too high",
			config:  Config{Host: "127.0.0.1", Port: 70000, ShutdownTimeout: 5 * time.Second},
			wantErr: constants.ErrInvalidValue,
		},
		{
			name:    "shutdown timeout too small",
			config:  Config{Host: "127.0.0.1", Port: 8080, ShutdownTimeout: 100 * time.Millisecond},
			wantErr: constants.ErrInvalidValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, must := assert.New(t), require.New(t)

			_, err := Run(context.Background(), testLogger(), tt.config)
			must.Error(err)
			want.ErrorIs(err, tt.wantErr)
			want.ErrorContains(err, tt.wantErr.Error())
		})
	}
}

func TestRun_ServesUntilCancelled(t *testing.T) {
	t.Parallel()
	want, must := assert.New(t), require.New(t)

	cfg := Config{Host: "127.0.0.1", Port: 0, ShutdownTimeout: 5 * time.Second}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		_, err := Run(ctx, testLogger(), cfg)
		done <- err
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		must.NoError(err)
	case <-time.After(10 * time.Second):
		want.Fail("serve did not stop after cancellation")
	}
}
