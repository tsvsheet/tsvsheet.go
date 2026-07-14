package set

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gomatic/template.cli/internal/constants"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

func TestRun(t *testing.T) {
	t.Parallel()
	tests := []struct {
		wantErr error
		name    string
		args    []string
		config  Config
	}{
		{name: "create new key", config: Config{}, args: []string{"new.key", "value"}},
		{name: "update existing key", config: Config{}, args: []string{"app.name", "renamed"}},
		{name: "dry-run new key", config: Config{DryRunEnabled: true}, args: []string{"new.key", "value"}},
		{name: "dry-run existing key", config: Config{DryRunEnabled: true}, args: []string{"app.name", "renamed"}},
		{name: "missing key and value", config: Config{}, args: []string{}, wantErr: constants.ErrMissingArgument},
		{name: "missing value", config: Config{}, args: []string{"key"}, wantErr: constants.ErrMissingArgument},
		{name: "empty key", config: Config{}, args: []string{"", "value"}, wantErr: constants.ErrInvalidName},
		{name: "empty value", config: Config{}, args: []string{"key", ""}, wantErr: constants.ErrInvalidValue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, must := assert.New(t), require.New(t)

			_, err := Run(context.Background(), testLogger(), tt.config, tt.args...)

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
