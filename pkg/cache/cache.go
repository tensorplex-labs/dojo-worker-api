package cache

import (
	"context"
	"crypto/tls"
	"dojo-api/utils"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/redis/rueidis"

	"github.com/rs/zerolog/log"
)

type RedisConfig struct {
	Address  string
	Password string
	Db       int
}

type Cache struct {
	redis rueidis.Client
}

var (
	instance *Cache
	once     sync.Once
)

func GetCacheInstance() *Cache {
	once.Do(func() {
		host := utils.LoadDotEnv("REDIS_HOST")
		port := utils.LoadDotEnv("REDIS_PORT")

		redis_url := host + ":" + port
		clientOpts := rueidis.ClientOption{
			InitAddress:  []string{redis_url},
			DisableCache: true,
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
		redisClient, err := rueidis.NewClient(clientOpts)
		if err != nil {
			log.Panic().Err(err).Msg("Failed to initialise Redis connection!")
		}
		log.Info().Msgf("Successfully connected to Redis")
		instance = &Cache{redis: redisClient}
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
	err := c.redis.Do(
		ctx,
		c.redis.B().Set().Key(key).Value(value.(string)).Ex(expiration).Build(),
	).Error()
	if err != nil {
		log.Error().Err(err).Str("key", key).Interface("value", value).Msg("Failed to write to Redis ...")
		return err
	}
	return nil
}

func (rc *Cache) Get(key string) (string, error) {
	ctx := context.Background()
	val, err := rc.redis.Do(ctx, rc.redis.B().Get().Key(key).Build()).AsBytes()
	if err == rueidis.Nil {
		return "", err
	} else if err != nil {
		log.Panic().Err(err).Msg("Failed to get from Redis ...")
	}
	return string(val), err
}

func (c *Cache) Shutdown() {
	c.redis.Close()
	log.Info().Msg("Successfully closed Redis connection")
}
