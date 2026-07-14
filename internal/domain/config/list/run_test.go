package list

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gomatic/template.cli/internal/config"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

func TestRun(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		config    Config
		wantKey   config.Key
		wantCount int
	}{
		{name: "all entries", config: Config{}, wantCount: 6, wantKey: "app.name"},
		{name: "app prefix", config: Config{Prefix: "app."}, wantCount: 2, wantKey: "app.name"},
		{name: "no matches", config: Config{Prefix: "zzz."}, wantCount: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, must := assert.New(t), require.New(t)

			result, err := Run(context.Background(), testLogger(), tt.config)
			must.NoError(err)
			want.Len(result, tt.wantCount)

			if tt.wantKey != "" {
				want.Contains(result, tt.wantKey)
			}
		})
	}
}
