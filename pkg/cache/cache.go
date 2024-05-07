package cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type Cache struct {
	kvStore         sync.Map
	defaultExpireAt time.Duration
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
		defaultExpireAt: defaultExpires,
	}
	c.StartExpiryRoutine()
	return c
}

func (c *Cache) Set(key, value interface{}) {
	now := time.Now().Unix()
	c.kvStore.Store(key, cacheItem{value: value, expiresAt: now + int64(c.defaultExpireAt.Seconds())})
	log.Info().Interface("key", key).Interface("value", value).Msg("Setting key value pair")

}

// Get returns the value associated with the key and a boolean value indicating
// whether the key was found.
func (c *Cache) Get(key interface{}) (interface{}, error) {
	item, ok := c.kvStore.Load(key)
	if !ok {
		return nil, fmt.Errorf("key not found")
	}

	cachedItem, ok := item.(cacheItem)
	if !ok {
		return nil, fmt.Errorf("type assertion to cacheItem failed")
	}

	if time.Now().Unix() > cachedItem.expiresAt {
		return nil, fmt.Errorf("key expired")
	}
	return cachedItem.value, nil
}

func (c *Cache) SetWithExpire(key, value interface{}, expiration time.Duration) error {
	if expiration <= 0 {
		return fmt.Errorf("expiration time must be greater than 0")
	}

	expiresAt := time.Now().UTC().Unix() + int64(expiration.Seconds())
	if item, ok := c.kvStore.Load(key); ok {
		cachedItem, ok := item.(cacheItem)
		if !ok {
			return fmt.Errorf("type assertion to cacheItem failed")
		}

		// if item exists, set the new expiration time
		cachedItem.expiresAt = expiresAt
		cachedItem.value = value
		c.kvStore.Store(key, cacheItem{
			value:     value,
			expiresAt: expiresAt,
		})
		return nil
	}

	// add the new item with the specified expiration time.
	c.kvStore.Store(key, cacheItem{
		value:     value,
		expiresAt: expiresAt,
	})
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

	c.kvStore.Range(func(key, value interface{}) bool {
		item, ok := value.(cacheItem)
		if !ok {
			log.Error().Interface("key", key).Msg("Type assertion to cacheItem failed during eviction")
			return true
		}

		if item.expiresAt < now {
			log.Warn().Int64("triggerTime", now).Interface("key", key).Int64("expireAt", item.expiresAt).Msgf("Evicting key: %v", key)
			c.kvStore.Delete(key)
		}
		return true
	})
}

func (c *Cache) Keys() []interface{} {
	keys := make([]interface{}, 0)
	c.kvStore.Range(func(key, value interface{}) bool {
		keys = append(keys, key)
		return true
	})
	return keys
}

func (c *Cache) ShowAll() {
	log.Info().Msg("Cache entries START")
	log.Info().Msg("Cache entries START")
	log.Info().Msg("Cache entries START")

	c.kvStore.Range(func(key, value interface{}) bool {
		item, ok := value.(cacheItem)
		if !ok {
			log.Error().Interface("key", key).Msg("Type assertion to cacheItem failed")
			return true
		}
		log.Info().Interface("Key", key).Interface("Value", item.value).Int64("expireAt", item.expiresAt).Msg("Cache entry details")
		return true
	})

	log.Info().Msg("Cache entries END")
	log.Info().Msg("Cache entries END")
	log.Info().Msg("Cache entries END")
}
