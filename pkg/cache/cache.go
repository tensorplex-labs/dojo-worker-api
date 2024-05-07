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
	mutex           sync.Mutex
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
	c.mutex.Lock()
	c.kvStore.Store(key, cacheItem{value: value, expiresAt: now + int64(c.defaultExpireAt.Seconds())})
	c.mutex.Unlock()
	log.Info().Interface("key", key).Interface("value", value).Msg("Setting key value pair")

}

// Get returns the value associated with the key and a boolean value indicating
// whether the key was found.
func (c *Cache) Get(key interface{}) (interface{}, error) {
	c.mutex.Lock()
	item, ok := c.kvStore.Load(key)
	c.mutex.Unlock()
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

	expiresAt := time.Now().Unix() + int64(expiration.Seconds())

	// Create the new cache item
	newItem := cacheItem{
		value:     value,
		expiresAt: expiresAt,
	}

	// Store the new item, whether or not the key already exists
	c.mutex.Lock()
	c.kvStore.Store(key, newItem)
	c.mutex.Unlock()
	return nil
}

func (c *Cache) StartExpiryRoutine() {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			c.mutex.Lock()
			c.evictExpiredItems()
			c.mutex.Unlock()
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
	c.mutex.Lock()
	keys := make([]interface{}, 0)
	c.kvStore.Range(func(key, value interface{}) bool {
		keys = append(keys, key)
		return true
	})
	c.mutex.Unlock()
	return keys
}

func (c *Cache) ShowAll() {
	log.Info().Msg("Cache entries START")
	log.Info().Msg("Cache entries START")
	log.Info().Msg("Cache entries START")

	c.mutex.Lock()
	c.kvStore.Range(func(key, value interface{}) bool {
		item, ok := value.(cacheItem)
		if !ok {
			log.Error().Interface("key", key).Msg("Type assertion to cacheItem failed")
			return true
		}
		log.Info().Interface("Key", key).Interface("Value", item.value).Int64("expireAt", item.expiresAt).Msg("Cache entry details")
		return true
	})
	c.mutex.Unlock()

	log.Info().Msg("Cache entries END")
	log.Info().Msg("Cache entries END")
	log.Info().Msg("Cache entries END")
}
