package identity

import (
	"sync"
	"time"

	"github.com/ylighgh/prometheus-cloud-sd/internal/core"
)

type Key struct {
	Provider    core.Provider
	AccountName string
	AccessKeyID string
}

type Identity struct {
	Provider   core.Provider
	AccountID  string
	ResolvedAt time.Time
}

type Cache interface {
	Get(key Key) (Identity, bool)
	Set(key Key, value Identity)
}

type MemoryCache struct {
	mu      sync.RWMutex
	entries map[Key]Identity
}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{entries: map[Key]Identity{}}
}

func (c *MemoryCache) Get(key Key) (Identity, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, ok := c.entries[key]
	return value, ok
}

func (c *MemoryCache) Set(key Key, value Identity) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = value
}
