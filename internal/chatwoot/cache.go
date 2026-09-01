package chatwoot

import (
	"sync"
	"time"
)

type cacheItem struct {
	value any
	exp   time.Time
}

type ttlCache struct {
	mu    sync.Mutex
	items map[string]cacheItem
	ttl   time.Duration
}

func newTTLCache(ttl time.Duration) *ttlCache {
	return &ttlCache{
		items: make(map[string]cacheItem),
		ttl:   ttl,
	}
}

func cached[T any](c *Client, key string, fn func() (T, error)) (T, error) {
	var zero T
	if c == nil || c.cache == nil {
		return fn()
	}
	c.cache.mu.Lock()
	if item, ok := c.cache.items[key]; ok && time.Now().Before(item.exp) {
		v, ok := item.value.(T)
		c.cache.mu.Unlock()
		if ok {
			return v, nil
		}
	} else {
		c.cache.mu.Unlock()
	}
	v, err := fn()
	if err != nil {
		return zero, err
	}
	c.cache.mu.Lock()
	c.cache.items[key] = cacheItem{value: v, exp: time.Now().Add(c.cache.ttl)}
	c.cache.mu.Unlock()
	return v, nil
}
