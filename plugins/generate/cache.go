package generate

import (
	"sync"
	"time"
)

const (
	defaultTTL      = 30 * time.Minute
	defaultMaxItems = 1000
	sweepInterval   = 1 * time.Minute
)

type CacheEntry struct {
	Files     []RenderedFile
	Kind      string
	CreatedAt time.Time
}

type GenerationCache struct {
	mu       sync.RWMutex
	entries  map[string]*CacheEntry
	lruOrder []string
	ttl      time.Duration
	maxItems int
	stopCh   chan struct{}
}

func NewGenerationCache() *GenerationCache {
	c := &GenerationCache{
		entries:  make(map[string]*CacheEntry),
		ttl:      defaultTTL,
		maxItems: defaultMaxItems,
		stopCh:   make(chan struct{}),
	}
	go c.sweepLoop()
	return c
}

func (c *GenerationCache) Set(id, kind string, files []RenderedFile) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) >= c.maxItems {
		oldest := c.lruOrder[0]
		c.lruOrder = c.lruOrder[1:]
		delete(c.entries, oldest)
	}

	c.entries[id] = &CacheEntry{
		Files:     files,
		Kind:      kind,
		CreatedAt: time.Now(),
	}
	c.lruOrder = append(c.lruOrder, id)
}

func (c *GenerationCache) Get(id string) (*CacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[id]
	if !ok {
		return nil, false
	}
	if time.Since(entry.CreatedAt) > c.ttl {
		return nil, false
	}
	return entry, true
}

func (c *GenerationCache) sweepLoop() {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.sweep()
		case <-c.stopCh:
			return
		}
	}
}

func (c *GenerationCache) sweep() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	var remaining []string
	for _, id := range c.lruOrder {
		entry, ok := c.entries[id]
		if !ok {
			continue
		}
		if now.Sub(entry.CreatedAt) > c.ttl {
			delete(c.entries, id)
		} else {
			remaining = append(remaining, id)
		}
	}
	c.lruOrder = remaining
}

func (c *GenerationCache) Stop() {
	close(c.stopCh)
}
