package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	store "github.com/gomatic/template.cli/internal/config"
	"github.com/gomatic/template.cli/internal/constants"
)

func TestKeyFrom(t *testing.T) {
	t.Parallel()
	tests := []struct {
		wantErr error
		name    string
		wantKey store.Key
		args    []string
	}{
		{name: "valid key", args: []string{"app.name"}, wantKey: "app.name"},
		{name: "extra args ignored", args: []string{"app.name", "extra"}, wantKey: "app.name"},
		{name: "missing key", args: []string{}, wantErr: constants.ErrMissingArgument},
		{name: "empty key", args: []string{""}, wantErr: constants.ErrInvalidName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			want, must := assert.New(t), require.New(t)

			key, err := KeyFrom(tt.args...)

			if tt.wantErr != nil {
				want.ErrorIs(err, tt.wantErr)
				return
			}
			must.NoError(err)
			want.Equal(tt.wantKey, key)
		})
	}
}

func TestPairFrom(t *testing.T) {
	t.Parallel()
	tests := []struct {
		wantErr   error
		name      string
		wantKey   store.Key
		wantValue store.Value
		args      []string
	}{
		{name: "valid pair", args: []string{"app.name", "renamed"}, wantKey: "app.name", wantValue: "renamed"},
		{name: "missing key and value", args: []string{}, wantErr: constants.ErrMissingArgument},
		{name: "missing value", args: []string{"app.name"}, wantErr: constants.ErrMissingArgument},
		{name: "empty key", args: []string{"", "renamed"}, wantErr: constants.ErrInvalidName},
		{name: "empty value", args: []string{"app.name", ""}, wantErr: constants.ErrInvalidValue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			want, must := assert.New(t), require.New(t)

			key, value, err := PairFrom(tt.args...)

			if tt.wantErr != nil {
				want.ErrorIs(err, tt.wantErr)
				return
			}
			must.NoError(err)
			want.Equal(tt.wantKey, key)
			want.Equal(tt.wantValue, value)
		})
	}
}
