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

// func GetCacheInstance() *Cache {
// 	once.Do(func() {
// 		mu.Lock()
// 		defer mu.Unlock()

// 		host := utils.LoadDotEnv("REDIS_HOST")
// 		port := utils.LoadDotEnv("REDIS_PORT")

// 		redis_url := host + ":" + port
// 		clientOpts := rueidis.ClientOption{
// 			InitAddress:  []string{redis_url},
// 			DisableCache: true,
// 		}
// 		if runtime_env := utils.LoadDotEnv("RUNTIME_ENV"); runtime_env == "aws" {
// 			clientOpts.TLSConfig = &tls.Config{
// 				MinVersion: tls.VersionTLS12,
// 			}
// 		}

// 		if username, usernameSet := os.LookupEnv("REDIS_USERNAME"); usernameSet {
// 			clientOpts.Username = username
// 		}
// 		if password, passwordSet := os.LookupEnv("REDIS_PASSWORD"); passwordSet {
// 			clientOpts.Password = password
// 		}
// 		redisClient, err := rueidis.NewClient(clientOpts)
// 		if err != nil {
// 			log.Fatal().Err(err).Msg("Failed to initialize Redis connection!")
// 		}

// 		// Ping Redis to test the connection
// 		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 		defer cancel()

// 		err = redisClient.Do(ctx, redisClient.B().Ping().Build()).Error()
// 		if err != nil {
// 			log.Fatal().Err(err).Msg("Failed to ping Redis using rueidis client")
// 		}

// 		log.Info().Msgf("Successfully connected to Redis and ping succeeded")
// 		instance = &Cache{Redis: redisClient}
// 	})
// 	return instance
// }

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
