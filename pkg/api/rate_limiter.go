package api

import (
	"sync"
	"time"

	"dojo-api/pkg/cache"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	limiter "github.com/ulule/limiter/v3"
	sredis "github.com/ulule/limiter/v3/drivers/store/redis"
)

var (
	limiters sync.Map
	initOnce sync.Once
)

type RateLimiterKey string

const (
	WorkerRateLimiterKey    RateLimiterKey = "dojo_worker_api:limiter:worker"
	WriteTaskRateLimiterKey RateLimiterKey = "dojo_worker_api:limiter:task_write"
	ReadTaskRateLimiterKey  RateLimiterKey = "dojo_worker_api:limiter:task_read"
	MetricsRateLimiterKey   RateLimiterKey = "dojo_worker_api:limiter:metrics"
	MinerRateLimiterKey     RateLimiterKey = "dojo_miner_api:limiter:miner"
)

type LimiterConfig struct {
	key    RateLimiterKey
	rate   limiter.Rate
	prefix string
}

func init() {
	InitializeLimiters()
}

func InitializeLimiters() {
	initOnce.Do(func() {
		cache := cache.GetCacheInstance()

		limiterConfigs := []LimiterConfig{
			{
				key:    WorkerRateLimiterKey,
				rate:   limiter.Rate{Period: 1 * time.Hour, Limit: 50},
				prefix: string(WorkerRateLimiterKey),
			},
			{
				key:    MinerRateLimiterKey,
				rate:   limiter.Rate{Period: 1 * time.Hour, Limit: 360},
				prefix: string(MinerRateLimiterKey),
			},
			{
				key:    WriteTaskRateLimiterKey,
				rate:   limiter.Rate{Period: 1 * time.Hour, Limit: 12},
				prefix: string(WriteTaskRateLimiterKey),
			},
			{
				key:    ReadTaskRateLimiterKey,
				rate:   limiter.Rate{Period: 1 * time.Hour, Limit: 120},
				prefix: string(ReadTaskRateLimiterKey),
			},
			{
				key:    MetricsRateLimiterKey,
				rate:   limiter.Rate{Period: 1 * time.Hour, Limit: 1800},
				prefix: string(MetricsRateLimiterKey),
			},
		}

		for _, config := range limiterConfigs {
			store, err := sredis.NewStoreWithOptions(&cache.Redis, limiter.StoreOptions{
				Prefix:   config.prefix,
				MaxRetry: 3,
			})
			if err != nil {
				log.Error().Err(err).Str("prefix", config.prefix).Msg("Failed to create rate limiter store")
				continue
			}
			limiters.Store(config.key, limiter.New(store, config.rate))
		}
	})
}

func getRateLimiterMiddleware(key RateLimiterKey) gin.HandlerFunc {
	return func(c *gin.Context) {
		limiterInstance, ok := limiters.Load(key)
		if !ok {
			log.Error().Str("key", string(key)).Msg("Rate limiter not found")
			c.Next()
			return
		}

		limiter := limiterInstance.(*limiter.Limiter)
		ip := c.ClientIP()
		limiterCtx, err := limiter.Get(c, ip)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get rate limiter")
			c.AbortWithStatus(500)
			return
		}

		if limiterCtx.Reached {
			c.AbortWithStatus(429) // Too Many Requests
			return
		}

		c.Next()
	}
}

func GenerousRateLimiter() gin.HandlerFunc {
	return getRateLimiterMiddleware(WorkerRateLimiterKey)
}

func MinerRateLimiter() gin.HandlerFunc {
	return getRateLimiterMiddleware(MinerRateLimiterKey)
}

func WriteTaskRateLimiter() gin.HandlerFunc {
	return getRateLimiterMiddleware(WriteTaskRateLimiterKey)
}

func ReadTaskRateLimiter() gin.HandlerFunc {
	return getRateLimiterMiddleware(ReadTaskRateLimiterKey)
}

func MetricsRateLimiter() gin.HandlerFunc {
	return getRateLimiterMiddleware(MetricsRateLimiterKey)
}
