package cache

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestDiscardedCacheCanBeCollected(t *testing.T) {
	collected := make(chan struct{})
	func() {
		c := New[int](2, time.Hour)
		c.Set("key", 1)
		runtime.SetFinalizer(c, func(*Cache[int]) { close(collected) })
	}()
	runtime.GC()
	select {
	case <-collected:
	case <-time.After(5 * time.Second):
		t.Fatal("discarded cache remains reachable, possibly from a cleanup worker")
	}
}

func TestGetExpiresBeforeAnyCleanup(t *testing.T) {
	c := New[string](2, time.Minute)
	c.cache["expired"] = cacheItemInternal[string]{obj: "stale", Timestamp: time.Now().Add(-2 * time.Minute)}
	if value, ok := c.Get("expired"); ok {
		t.Fatalf("expired entry returned: %q", value)
	}
	if len(c.cache) != 0 {
		t.Fatal("expired entry was retained after access")
	}
}

func TestReplacementDoesNotEvictOtherEntries(t *testing.T) {
	c := New[string](2, time.Hour)
	c.Set("old", "old")
	c.Set("other", "other")
	c.Set("old", "updated")
	for key, want := range map[string]string{"old": "updated", "other": "other"} {
		if got, ok := c.Get(key); !ok || got != want {
			t.Fatalf("Get(%q) = %q, %t, want %q", key, got, ok, want)
		}
	}
}

func TestExpiredEntriesRemovedBeforeLiveEviction(t *testing.T) {
	c := New[string](2, time.Minute)
	c.Set("live", "keep")
	c.cache["expired"] = cacheItemInternal[string]{obj: "stale", Timestamp: time.Now().Add(-2 * time.Minute)}
	c.Set("new", "new")
	if _, exists := c.cache["expired"]; exists {
		t.Fatal("expired entry survived capacity cleanup")
	}
	if got, ok := c.Get("live"); !ok || got != "keep" {
		t.Fatal("live entry evicted instead of expired entry")
	}
}

func TestDisabledCache(t *testing.T) {
	for _, c := range []*Cache[string]{New[string](0, time.Minute), New[string](1, 0), New[string](1, -time.Minute)} {
		c.Set("key", "value")
		if _, ok := c.Get("key"); ok {
			t.Fatal("disabled cache retained a value")
		}
	}
}

func TestEmptyKeyStillObeysCapacity(t *testing.T) {
	c := New[int](1, time.Minute)
	c.Set("", 1)
	c.Set("next", 2)
	if len(c.cache) != 1 {
		t.Fatalf("cache exceeds capacity: %d entries", len(c.cache))
	}
	if _, ok := c.Get(""); ok {
		t.Fatal("old empty key was not evicted")
	}
}

func TestConcurrentAccess(t *testing.T) {
	c := New[int](8, time.Minute)
	var workers sync.WaitGroup
	for i := range 8 {
		workers.Go(func() {
			for range 100 {
				c.Set("key", i)
				c.Get("key")
				c.Remove("key")
			}
		})
	}
	workers.Wait()
}
