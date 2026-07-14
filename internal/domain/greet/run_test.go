package greet

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gomatic/template.cli/internal/constants"
	"github.com/gomatic/template.cli/internal/greeting"
)

func TestRun(t *testing.T) {
	t.Parallel()
	tests := []struct {
		wantErr error
		name    string
		want    greeting.Message
		args    []string
		config  Config
	}{
		{
			name:   "simple greeting",
			config: Config{Greeting: "Hello"},
			args:   []string{"World"},
			want:   "Hello, World!",
		},
		{
			name:   "empty greeting defaults to Hello",
			config: Config{Greeting: ""},
			args:   []string{"World"},
			want:   "Hello, World!",
		},
		{
			name:   "uppercase greeting",
			config: Config{Greeting: "Hello", UppercaseEnabled: true},
			args:   []string{"World"},
			want:   "HELLO, WORLD!",
		},
		{
			name:   "enthusiast mode",
			config: Config{Greeting: "Hello", EnthusiastEnabled: true},
			args:   []string{"World"},
			want:   "Hello, World!!!",
		},
		{
			name:   "repeat greeting",
			config: Config{Greeting: "Hello", Repeat: 3},
			args:   []string{"World"},
			want:   "Hello, World!\nHello, World!\nHello, World!",
		},
		{
			name:   "repeat less than minimum defaults to one",
			config: Config{Greeting: "Hello", Repeat: 0},
			args:   []string{"World"},
			want:   "Hello, World!",
		},
		{
			name:   "combined uppercase and enthusiast",
			config: Config{Greeting: "Hello", UppercaseEnabled: true, EnthusiastEnabled: true},
			args:   []string{"World"},
			want:   "HELLO, WORLD!!!",
		},
		{
			name:    "empty name",
			config:  Config{Greeting: "Hello"},
			args:    []string{""},
			wantErr: constants.ErrInvalidName,
		},
		{
			name:    "missing name",
			config:  Config{Greeting: "Hello"},
			args:    []string{},
			wantErr: constants.ErrMissingArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, must := assert.New(t), require.New(t)

			logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
			result, err := Run(context.Background(), logger, tt.config, tt.args...)

			if tt.wantErr != nil {
				must.Error(err)
				want.ErrorIs(err, tt.wantErr)
				want.ErrorContains(err, tt.wantErr.Error())
				return
			}

			must.NoError(err)
			want.Equal(tt.want, result.Message)
		})
	}
}
