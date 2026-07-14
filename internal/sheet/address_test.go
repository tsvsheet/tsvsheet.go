package sheet_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/constants"
	"github.com/uplang/tsvsheet.go/internal/sheet"
)

func TestParseAddress(t *testing.T) {
	t.Parallel()

	cases := map[string]sheet.Address{
		"A1":   {Row: 0, Col: 0},
		"B2":   {Row: 1, Col: 1},
		"Z1":   {Row: 0, Col: 25},
		"AA1":  {Row: 0, Col: 26},
		"F4":   {Row: 3, Col: 5},
		"AB10": {Row: 9, Col: 27},
	}
	for src, want := range cases {
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			got, err := sheet.ParseAddress(src)
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func TestParseAddress_Invalid(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"no letters":    "1",
		"no digits":     "A",
		"row zero":      "A0",
		"trailing junk": "A1x",
		"empty":         "",
		"lowercase":     "a1",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := sheet.ParseAddress(src)
			require.Error(t, err)
			assert.ErrorIs(t, err, constants.ErrInvalidValue)
		})
	}
}

func TestAddress_String(t *testing.T) {
	t.Parallel()

	cases := map[string]sheet.Address{
		"A1":   {Row: 0, Col: 0},
		"Z1":   {Row: 0, Col: 25},
		"AA1":  {Row: 0, Col: 26},
		"F4":   {Row: 3, Col: 5},
		"AB10": {Row: 9, Col: 27},
	}
	for want, addr := range cases {
		t.Run(want, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, addr.String())
		})
	}
}

func TestAddress_RoundTrip(t *testing.T) {
	t.Parallel()

	for _, s := range []string{"A1", "Z26", "AA100", "ZZ1", "AAA5"} {
		addr, err := sheet.ParseAddress(s)
		require.NoError(t, err)
		assert.Equal(t, s, addr.String())
	}
}
