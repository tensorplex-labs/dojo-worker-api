package cache

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"sync"
	"time"

	"dojo-api/utils"

	"github.com/redis/go-redis/v9"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/rs/zerolog/log"
)

type RedisConfig struct {
	Address  string
	Password string
	Db       int
}

type Cache struct {
	Redis redis.Client
	Keys  CacheKeys
}

var (
	instance *Cache
	once     sync.Once
	mu       sync.Mutex
)

type CacheKey string

// CacheKeys holds all cache key constants
type CacheKeys struct {
	// Task cache keys
	TaskById      CacheKey
	TasksByWorker CacheKey

	// Task Result cache keys
	TaskResultByTaskAndWorker CacheKey
	TaskResultByWorker        CacheKey
	TaskResultsTotal          CacheKey
	CompletedTasksTotal       CacheKey
	// Worker cache keys
	WorkerByWallet CacheKey
	WorkerCount    CacheKey

	// Subscription cache keys
	SubByHotkey CacheKey
	SubByKey    CacheKey
}

// Default cache keys
var cacheKeys = CacheKeys{
	// Task cache keys
	TaskById:      "task",
	TasksByWorker: "task:worker",

	// Task Result cache keys
	TaskResultByTaskAndWorker: "tr:task:worker",
	TaskResultByWorker:        "tr:worker",
	TaskResultsTotal:          "metrics:tr:total",
	CompletedTasksTotal:       "metrics:completed_tasks:total",

	// Worker cache keys
	WorkerByWallet: "worker:wallet",
	WorkerCount:    "worker:count",

	// Subscription cache keys
	SubByHotkey: "sub:hotkey",
	SubByKey:    "sub:key",
}

var cacheExpirations = map[CacheKey]time.Duration{
	cacheKeys.TaskById:                  5 * time.Minute,
	cacheKeys.TasksByWorker:             2 * time.Minute,
	cacheKeys.TaskResultByTaskAndWorker: 10 * time.Minute,
	cacheKeys.TaskResultByWorker:        10 * time.Minute,
	cacheKeys.WorkerByWallet:            5 * time.Minute,
	cacheKeys.WorkerCount:               1 * time.Minute,
	cacheKeys.SubByHotkey:               5 * time.Minute,
	cacheKeys.SubByKey:                  5 * time.Minute,
}

func GetCacheInstance() *Cache {
	once.Do(func() {
		mu.Lock()
		defer mu.Unlock()
		host := utils.LoadDotEnv("REDIS_HOST")
		port := utils.LoadDotEnv("REDIS_PORT")

		redis_url := host + ":" + port
		clientOpts := &redis.Options{
			Addr: redis_url,
		}

		if runtime_env := utils.LoadDotEnv("RUNTIME_ENV"); runtime_env == "aws" {
			clientOpts.TLSConfig = &tls.Config{
				MinVersion: tls.VersionTLS12,
			}
		}

		if username, usernameSet := os.LookupEnv("REDIS_USERNAME"); usernameSet {
			clientOpts.Username = username
		}
		if password, passwordSet := os.LookupEnv("REDIS_PASSWORD"); passwordSet {
			clientOpts.Password = password
		}
		redisClient := redis.NewClient(clientOpts)

		// Ping Redis to check the connection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		res, err := redisClient.Ping(ctx).Result()
		log.Info().Msgf("Redis ping result: %v", res)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to ping Redis using default client")
		}

		log.Info().Msgf("Successfully connected to Redis and ping succeeded")
		instance = &Cache{
			Redis: *redisClient,
			Keys:  cacheKeys,
		}
	})
	return instance
}

// GetCacheExpiration returns the expiration time for a given cache key
func (c *Cache) GetCacheExpiration(key CacheKey) time.Duration {
	if duration, exists := cacheExpirations[key]; exists {
		return duration
	}
	return 5 * time.Minute // default expiration
}

// BuildCacheKey builds a cache key with the given prefix and components
func (c *Cache) BuildCacheKey(prefix CacheKey, components ...string) string {
	key := string(prefix)
	for _, component := range components {
		key += ":" + component
	}
	return key
}

func (c *Cache) SetWithExpire(key string, value interface{}, expiration time.Duration) error {
	switch v := value.(type) {
	case string:
		// do nothing, it's expected
	case []byte:
		value = string(v)
	default:
		fmt.Println("Unknown type, ignoring and storing directly in Redis")
	}

	ctx := context.Background()
	err := c.Redis.Set(ctx, key, value.(string), expiration).Err()
	if err != nil {
		log.Error().Err(err).Str("key", key).Interface("value", value).Msg("Failed to write to Redis ...")
		return err
	}
	return nil
}

func (c *Cache) Get(key string) (string, error) {
	ctx := context.Background()
	// val, err := rc.Redis.Do(ctx, rc.Redis.B().Get().Key(key).Build()).AsBytes()
	val, err := c.Redis.Get(ctx, key).Bytes()
	if err == redis.Nil {
		log.Debug().Str("key", key).Msg("Key not found in Redis")
		return "", err
	} else if err != nil {
		log.Panic().Err(err).Msg("Failed to get from Redis ...")
	}
	return string(val), err
}

func (c *Cache) Shutdown() {
	c.Redis.Close()
	log.Info().Msg("Successfully closed Redis connection")
}

// GetCacheValue retrieves and unmarshals data from cache using MessagePack
func (c *Cache) GetCacheValue(key string, value interface{}) error {
	cachedData, err := c.Get(key)
	if err != nil || cachedData == "" {
		return fmt.Errorf("cache miss for key: %s", key)
	}

	log.Info().Msgf("Cache hit for key: %s", key)
	return msgpack.Unmarshal([]byte(cachedData), value)
}

// SetCacheValue marshals and stores data in cache using MessagePack
func (c *Cache) SetCacheValue(key string, value interface{}) error {
	dataBytes, err := msgpack.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	expiration := c.GetCacheExpiration(CacheKey(key))
	if err := c.SetWithExpire(key, dataBytes, expiration); err != nil {
		return fmt.Errorf("failed to set cache: %w", err)
	}

	log.Info().Msgf("Successfully set cache for key: %s", key)
	return nil
}

// Delete removes a single specific cache key
// Example: cache.Delete("task:worker:123")
func (c *Cache) Delete(key string) error {
	ctx := context.Background()
	deleted, err := c.Redis.Del(ctx, key).Result()
	if err != nil {
		log.Error().Err(err).Str("key", key).Msg("Failed to clean cache key")
		return fmt.Errorf("failed to delete key: %w", err)
	}

	if deleted > 0 {
		log.Info().Str("key", key).Msg("Clean up Cache")
	} else {
		log.Debug().Str("key", key).Msg("Cache key not found")
	}
	return nil
}

// DeleteByPattern removes all cache entries matching the given pattern
// Example: cache.DeleteByPattern("task:worker:*") - deletes all worker task caches
// Example: cache.DeleteByPattern("user:123:*") - deletes all user 123's caches
func (c *Cache) DeleteByPattern(pattern string) error {
	ctx := context.Background()
	keys, err := c.Redis.Keys(ctx, pattern).Result()
	if err != nil {
		log.Error().Err(err).Str("pattern", pattern).Msg("Failed to get keys")
		return fmt.Errorf("failed to get keys: %w", err)
	}

	if len(keys) > 0 {
		deleted, err := c.Redis.Del(ctx, keys...).Result()
		if err != nil {
			log.Error().Err(err).Msg("Failed to delete keys")
			return err
		}
		log.Info().Int64("deleted", deleted).Str("pattern", pattern).Msg("Clean up Cache")
	}
	return nil
}

// DeleteWithSuffix removes a single cache key built with prefix and suffix components
// Example: cache.DeleteWithSuffix(cache.TasksByWorker, "123") -> deletes "task:worker:123"
func (c *Cache) DeleteWithSuffix(prefix CacheKey, suffixes ...string) error {
	key := c.BuildCacheKey(prefix, suffixes...)
	return c.Delete(key)
}
