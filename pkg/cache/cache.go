package cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type Cache struct {
	kvStore         map[interface{}]cacheItem
	defaultExpireAt time.Duration
	rwMutex         sync.RWMutex
}

type cacheItem struct {
	value     interface{}
	expiresAt time.Time
}

func NewCache(initialSize int, defaultExpires time.Duration) *Cache {
	if defaultExpires <= 0 {
		log.Fatal().Msg("default expiration time must be greater than 0")
		return nil
	}
	c := &Cache{
		kvStore:         make(map[interface{}]cacheItem, initialSize),
		defaultExpireAt: defaultExpires,
	}
	c.StartExpiryRoutine()
	return c
}

func (c *Cache) Set(key, value interface{}) {
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	if _, ok := c.kvStore[key]; ok {
		// if item exists, skip adding it
		return
	}

	c.kvStore[key] = cacheItem{
		value:     value,
		expiresAt: time.Now().Add(c.defaultExpireAt),
	}
}

// Get returns the value associated with the key and a boolean value indicating
// whether the key was found.
func (c *Cache) Get(key interface{}) (interface{}, error) {
	c.rwMutex.RLock()
	defer c.rwMutex.RUnlock()

	item, ok := c.kvStore[key]
	if !ok {
		return nil, fmt.Errorf("key not found")
	}

	if time.Now().After(item.expiresAt) {
		return nil, fmt.Errorf("key expired")
	}
	return item.value, nil
}

func (c *Cache) SetWithExpire(key, value interface{}, expiration time.Duration) error {
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	if expiration <= 0 {
		return fmt.Errorf("expiration time must be greater than 0")
	}

	expiresAt := time.Now().Add(expiration)
	if item, ok := c.kvStore[key]; ok {
		// if item exists, set the new expiration time
		item.expiresAt = expiresAt
		c.kvStore[key] = item
		return nil
	}

	// add the new item with the specified expiration time.
	c.kvStore[key] = cacheItem{
		value:     value,
		expiresAt: expiresAt,
	}

	return nil
}

func (c *Cache) StartExpiryRoutine() {
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			c.evictExpiredItems()
		}
	}()
}

func (c *Cache) evictExpiredItems() {
	now := time.Now()
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	for key, item := range c.kvStore {
		if item.expiresAt.Before(now) {
			log.Debug().Time("triggerTime", now).Time("keyExpireAt", item.expiresAt).Msgf("Evicting key: %v", key)
			delete(c.kvStore, key)
		}
	}
}

func (c *Cache) Keys() []interface{} {
	c.rwMutex.RLock()
	defer c.rwMutex.RUnlock()

	keys := make([]interface{}, 0)
	for key := range c.kvStore {
		keys = append(keys, key)
	}
	return keys
}
