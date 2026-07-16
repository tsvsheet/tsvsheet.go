package importer

import (
	"sync"

	"github.com/uplang/tsvsheet.go/internal/sheet"
)

// cacheKey identifies a cached fetch by the (url, accept) pair the engine asked
// for — the same key the handshake is keyed on.
type cacheKey struct {
	url    sheet.ImportURL
	accept sheet.MediaType
}

// Cache is the frontend-owned, cross-pass import cache (ADR 0006 §6): it wraps a
// sheet.Fetcher and memoizes successes so ordinary and clock-ticker recomputes
// reuse fetched values with NO network, and only an explicit refresh (Clear)
// drops them. Errors are never cached — a transient failure is retried next
// pass.
//
// Its methods take pointer receivers (the sanctioned exception, like
// session.Session): Cache wraps mutable state — a map guarded by a mutex — that
// must not be copied, and serve fetches concurrently.
type Cache struct {
	inner sheet.Fetcher
	cache map[cacheKey]sheet.FetchResult
	mu    sync.Mutex
}

// NewCache wraps inner with a cross-pass memoizing cache.
func NewCache(inner sheet.Fetcher) *Cache {
	return &Cache{inner: inner, cache: map[cacheKey]sheet.FetchResult{}}
}

// Fetch returns the cached result for (url, accept) when present, otherwise
// delegates to the inner Fetcher and caches a success. It is safe for
// concurrent use.
func (c *Cache) Fetch(url sheet.ImportURL, accept sheet.MediaType) (sheet.FetchResult, error) {
	key := cacheKey{url: url, accept: accept}
	if res, ok := c.load(key); ok {
		return res, nil
	}
	res, err := c.inner.Fetch(url, accept)
	if err != nil {
		return sheet.FetchResult{}, err
	}
	c.store(key, res)
	return res, nil
}

// Clear drops every cached entry — the explicit "refresh imports" action.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = map[cacheKey]sheet.FetchResult{}
}

// load returns the cached result for key, if any.
func (c *Cache) load(key cacheKey) (sheet.FetchResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	res, ok := c.cache[key]
	return res, ok
}

// store records a successful fetch under key.
func (c *Cache) store(key cacheKey, res sheet.FetchResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = res
}
