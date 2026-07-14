package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// appNameKey and appNameValue are the seeded example pair these tests exercise;
// naming them once keeps the literals from repeating across cases.
const (
	appNameKey   Key   = "app.name"
	appNameValue Value = "template.cli"
)

func TestNewStore(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	store := NewStore()
	value, ok := store.Get(appNameKey)
	want.True(ok)
	want.Equal(appNameValue, value)
	want.Len(store.List(""), 6)
}

func TestStore_Get(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		key       Key
		wantValue Value
		wantOK    bool
	}{
		{name: "existing key", key: appNameKey, wantValue: appNameValue, wantOK: true},
		{name: "missing key", key: "missing", wantValue: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := assert.New(t)

			value, ok := NewStore().Get(tt.key)
			want.Equal(tt.wantOK, ok)
			want.Equal(tt.wantValue, value)
		})
	}
}

func TestStore_Set(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		key          Key
		value        Value
		wantPrevious Value
		wantExisted  bool
	}{
		{
			name:         "updates existing key",
			key:          appNameKey,
			value:        "renamed",
			wantPrevious: appNameValue,
			wantExisted:  true,
		},
		{name: "creates new key", key: "new.key", value: "fresh", wantPrevious: "", wantExisted: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := assert.New(t)

			store := NewStore()
			previous, existed := store.Set(tt.key, tt.value)
			want.Equal(tt.wantExisted, existed)
			want.Equal(tt.wantPrevious, previous)

			stored, ok := store.Get(tt.key)
			want.True(ok)
			want.Equal(tt.value, stored)
		})
	}
}

func TestStore_List(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		prefix    Prefix
		wantCount int
	}{
		{name: "all entries", prefix: "", wantCount: 6},
		{name: "app prefix", prefix: "app.", wantCount: 2},
		{name: "no matches", prefix: "zzz.", wantCount: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.New(t).Len(NewStore().List(tt.prefix), tt.wantCount)
		})
	}
}
