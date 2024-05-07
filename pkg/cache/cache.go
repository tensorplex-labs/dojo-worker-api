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
	expiresAt int64 // Unix timestamp
}

func NewCache(initialSize int, defaultExpires time.Duration) *Cache {
	if defaultExpires <= 0 {
		log.Info().Msg("default expiration time must be greater than 0")
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

	now := time.Now().Unix()
	if _, ok := c.kvStore[key]; ok {
		// if item exists, overwrite it
		c.kvStore[key] = cacheItem{
			value:     value,
			expiresAt: now + int64(c.defaultExpireAt.Seconds()),
		}
		return
	}

	c.kvStore[key] = cacheItem{
		value:     value,
		expiresAt: now + int64(c.defaultExpireAt.Seconds()),
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

	if time.Now().Unix() > item.expiresAt {
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

	expiresAt := time.Now().UTC().Unix() + int64(expiration.Seconds())
	if item, ok := c.kvStore[key]; ok {
		// if item exists, set the new expiration time
		item.expiresAt = expiresAt
		item.value = value
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
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			c.evictExpiredItems()
		}
	}()
}

func (c *Cache) evictExpiredItems() {
	now := time.Now().Unix()
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	for key, item := range c.kvStore {
		if item.expiresAt < now {
			log.Warn().Int64("triggerTime", now).Interface("key", key).Int64("expireAt", item.expiresAt).Msgf("Evicting key: %v", key)

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

func (c *Cache) ShowAll() {
	c.rwMutex.RLock()
	defer c.rwMutex.RUnlock()
	log.Info().Msg("Cache entries START")
	log.Info().Msg("Cache entries START")
	log.Info().Msg("Cache entries START")

	for key, item := range c.kvStore {
		log.Info().Interface("Key", key).Interface("Value", item.value).Int64("expireAt", item.expiresAt).Msg("Cache entry details")
	}

	log.Info().Msg("Cache entries END")
	log.Info().Msg("Cache entries END")
	log.Info().Msg("Cache entries END")
}
