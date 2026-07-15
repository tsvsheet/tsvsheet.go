package refresh_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uplang/tsvsheet.go/internal/refresh"
)

// mon1005 is a fixed instant: Monday 2026-01-05 10:05 UTC.
var mon1005 = time.Date(2026, 1, 5, 10, 5, 0, 0, time.UTC)

func TestEvery(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 5*time.Second, refresh.Every(5*time.Second)(mon1005))
}

func TestParse_DurationAndDisabled(t *testing.T) {
	t.Parallel()

	off, err := refresh.Parse("")
	require.NoError(t, err)
	assert.Nil(t, off) // empty → disabled

	zero, err := refresh.Parse("0s")
	require.NoError(t, err)
	assert.Nil(t, zero) // non-positive → disabled

	fixed, err := refresh.Parse("30s")
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, fixed(mon1005))
}

func TestParse_IsnowPattern(t *testing.T) {
	t.Parallel()

	// "+[30mn]" fires every 30 minutes; the next from 10:05 is 10:30 (25m).
	every30, err := refresh.Parse("+[30mn]")
	require.NoError(t, err)
	assert.Equal(t, 25*time.Minute, every30(mon1005))

	// A pattern whose bound is in the past has no next match → disabled tick.
	past, err := refresh.Parse("12 <2016")
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), past(mon1005))
}

func TestParse_MalformedPattern(t *testing.T) {
	t.Parallel()

	_, err := refresh.Parse("garbage!!!")
	require.Error(t, err)
}
