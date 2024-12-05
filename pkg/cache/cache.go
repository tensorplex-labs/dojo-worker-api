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
}

var (
	instance *Cache
	once     sync.Once
	mu       sync.Mutex
)

// CacheKey type for type-safe cache keys
type CacheKey string

const (
	// Task cache keys
	TaskById      CacheKey = "task"        // Single task by ID
	TasksByWorker CacheKey = "task:worker" // List of tasks by worker

	// Task Result cache keys
	TaskResultByTaskAndWorker CacheKey = "tr:task:worker"             // Task result by task ID and worker ID
	TaskResultByWorker        CacheKey = "tr:worker"                  // Task results by worker ID
	TaskResultsTotal          CacheKey = "metrics:task_results:total" // Total task results count

	// Worker cache keys
	WorkerByWallet CacheKey = "worker:wallet" // Worker by wallet address
	WorkerCount    CacheKey = "worker:count"  // Total worker count

	// Subscription cache keys
	SubByHotkey CacheKey = "sub:hotkey" // Subscription by hotkey
	SubByKey    CacheKey = "sub:key"    // Subscription by key
)

// CacheConfig defines cache keys and their expiration times
var CacheConfig = map[CacheKey]time.Duration{
	TaskById:                  5 * time.Minute,
	TasksByWorker:             2 * time.Minute,
	TaskResultByTaskAndWorker: 10 * time.Minute,
	TaskResultByWorker:        10 * time.Minute,
	WorkerByWallet:            5 * time.Minute,
	WorkerCount:               1 * time.Minute,
	SubByHotkey:               5 * time.Minute,
	SubByKey:                  5 * time.Minute,
}

// GetCacheExpiration returns the expiration time for a given cache key
func GetCacheExpiration(key CacheKey) time.Duration {
	if duration, exists := CacheConfig[key]; exists {
		return duration
	}
	return 5 * time.Minute // default expiration
}

// BuildCacheKey builds a cache key with the given prefix and components
func BuildCacheKey(prefix CacheKey, components ...string) string {
	key := string(prefix)
	for _, component := range components {
		key += ":" + component
	}
	return key
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
		instance = &Cache{Redis: *redisClient}
	})
	return instance
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
		log.Error().Err(err).Str("key", key).Msg("Key not found in Redis")
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

	expiration := GetCacheExpiration(CacheKey(key))
	if err := c.SetWithExpire(key, dataBytes, expiration); err != nil {
		return fmt.Errorf("failed to set cache: %w", err)
	}

	log.Info().Msgf("Successfully set cache for key: %s", key)
	return nil
}
