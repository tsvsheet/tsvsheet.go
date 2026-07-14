package get

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gomatic/template.cli/internal/config"
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
		config  Config
		want    config.Value
		args    []string
	}{
		{name: "existing key", config: Config{}, args: []string{"app.name"}, want: "template.cli"},
		{
			name:   "missing key with default",
			config: Config{DefaultValue: "fallback"},
			args:   []string{"missing.key"},
			want:   "fallback",
		},
		{
			name:    "missing key without default",
			config:  Config{},
			args:    []string{"missing.key"},
			wantErr: constants.ErrNotFound,
		},
		{name: "empty key", config: Config{}, args: []string{""}, wantErr: constants.ErrInvalidName},
		{name: "missing argument", config: Config{}, args: []string{}, wantErr: constants.ErrMissingArgument},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, must := assert.New(t), require.New(t)

			result, err := Run(context.Background(), testLogger(), tt.config, tt.args...)

			if tt.wantErr != nil {
				must.Error(err)
				want.ErrorIs(err, tt.wantErr)
				want.ErrorContains(err, tt.wantErr.Error())
				return
			}

			must.NoError(err)
			want.Equal(tt.want, result.Value)
		})
	}
}
