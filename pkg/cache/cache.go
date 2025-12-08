package cache

import (
	"sync"
)

type Cache[K comparable, V any] struct {
	entries map[K]V
	mu      sync.RWMutex
}

func New[K comparable, V any]() *Cache[K, V] {
	c := &Cache[K, V]{
		mu:      sync.RWMutex{},
		entries: make(map[K]V),
	}
	return c
}

func (c *Cache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = value
}

func (c *Cache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[key]
	return entry, ok
}

func (c *Cache[K, V]) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

func (c *Cache[K, V]) Keys() []K {
	keys := make([]K, len(c.entries))
	i := 0
	for k := range c.entries {
		keys[i] = k
		i++
	}
	return keys
}
