package cache

import (
	"sync"
	"time"
)

type cacheItemInternal[T any] struct {
	obj       T
	Timestamp time.Time
}

type Cache[T any] struct {
	cache      map[string]cacheItemInternal[T]
	mutex      sync.RWMutex
	maxEntries int
	expiration time.Duration
}

func New[T any](maxEntries int, expiration time.Duration) *Cache[T] {
	cache := &Cache[T]{
		cache:      make(map[string]cacheItemInternal[T]),
		maxEntries: maxEntries,
		expiration: expiration,
	}

	return cache
}

// Expiration is checked on access and before eviction. The cache is bounded,
// so idle entries need no background worker: such a worker would otherwise keep
// both the cache and its owning preview/model alive after they are discarded.
// The caller must hold c.mutex for writing.
func (c *Cache[T]) cleanupExpired(now time.Time) {
	for key, entry := range c.cache {
		if now.Sub(entry.Timestamp) >= c.expiration {
			delete(c.cache, key)
		}
	}
}

func (c *Cache[T]) Get(key string) (T, bool) {
	c.mutex.RLock()
	entry, exists := c.cache[key]
	if exists && time.Since(entry.Timestamp) < c.expiration {
		c.mutex.RUnlock()
		return entry.obj, true
	}
	c.mutex.RUnlock()
	if exists {
		c.mutex.Lock()
		// A concurrent Set may have replaced the expired value.
		if current, ok := c.cache[key]; ok && time.Since(current.Timestamp) >= c.expiration {
			delete(c.cache, key)
		}
		c.mutex.Unlock()
	}
	var res T
	return res, false
}

func (c *Cache[T]) Set(key string, obj T) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.maxEntries <= 0 || c.expiration <= 0 {
		return
	}

	// Check if we need to evict entries
	if _, replacing := c.cache[key]; !replacing && len(c.cache) >= c.maxEntries {
		c.cleanupExpired(time.Now())
		if len(c.cache) >= c.maxEntries {
			c.evictOldest()
		}
	}

	c.cache[key] = cacheItemInternal[T]{
		obj:       obj,
		Timestamp: time.Now(),
	}
}

func (c *Cache[T]) Remove(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.cache, key)
}

// evictOldest removes the oldest entry from the cache
func (c *Cache[T]) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	found := false

	// Find the oldest entry
	for key, entry := range c.cache {
		if !found || entry.Timestamp.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.Timestamp
			found = true
		}
	}

	// Remove the oldest entry
	if found {
		delete(c.cache, oldestKey)
	}
}
