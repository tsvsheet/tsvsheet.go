package importer

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tsvsheet/go-tsvsheet"

	"github.com/tsvsheet/tsvsheet.go/internal/constants"
)

func TestCache_MissThenHitNoSecondFetch(t *testing.T) {
	t.Parallel()

	inner := &countingFetcher{res: tsvsheet.FetchResult{ContentType: cellMedia, Body: []byte("v")}}
	c := NewCache(inner)

	first, err := c.Fetch("https://x/a", cellMedia)
	require.NoError(t, err)
	assert.Equal(t, "v", string(first.Body))

	second, err := c.Fetch("https://x/a", cellMedia)
	require.NoError(t, err)
	assert.Equal(t, "v", string(second.Body))
	assert.Equal(t, int64(1), inner.calls.Load()) // second served from cache
}

func TestCache_ClearDropsEntries(t *testing.T) {
	t.Parallel()

	inner := &countingFetcher{res: tsvsheet.FetchResult{ContentType: cellMedia, Body: []byte("v")}}
	c := NewCache(inner)

	_, _ = c.Fetch("https://x/a", cellMedia)
	c.Clear()
	_, _ = c.Fetch("https://x/a", cellMedia)
	assert.Equal(t, int64(2), inner.calls.Load()) // refetched after Clear
}

func TestCache_KeyedByURLAndAccept(t *testing.T) {
	t.Parallel()

	inner := &countingFetcher{res: tsvsheet.FetchResult{ContentType: cellMedia, Body: []byte("v")}}
	c := NewCache(inner)

	_, _ = c.Fetch("https://x/a", cellMedia)
	_, _ = c.Fetch("https://x/b", cellMedia)               // different url
	_, _ = c.Fetch("https://x/a", "application/other+tsv") // different accept
	assert.Equal(t, int64(3), inner.calls.Load())
}

func TestCache_ErrorNotCached(t *testing.T) {
	t.Parallel()

	inner := &countingFetcher{err: constants.ErrImportFetch}
	c := NewCache(inner)

	_, err := c.Fetch("https://x/a", cellMedia)
	assert.ErrorIs(t, err, constants.ErrImportFetch)
	_, err = c.Fetch("https://x/a", cellMedia)
	assert.ErrorIs(t, err, constants.ErrImportFetch)
	assert.Equal(t, int64(2), inner.calls.Load()) // retried, not cached
}

func TestCache_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	inner := &countingFetcher{res: tsvsheet.FetchResult{ContentType: cellMedia, Body: []byte("v")}}
	c := NewCache(inner)

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			url := tsvsheet.ImportURL("https://x/a")
			if n%2 == 0 {
				c.Clear()
			}
			_, _ = c.Fetch(url, cellMedia)
		}(i)
	}
	wg.Wait()
}
