package api

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"dojo-api/pkg/blockchain"
	"dojo-api/pkg/cache"
	"dojo-api/utils"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	limiter "github.com/ulule/limiter/v3"
	sredis "github.com/ulule/limiter/v3/drivers/store/redis"
)

var (
	limiters sync.Map
	once     sync.Once
)

type RateLimiterKey string

const (
	WorkerRateLimiterKey    RateLimiterKey = "dojo_worker_api:limiter:worker"
	WriteTaskRateLimiterKey RateLimiterKey = "dojo_worker_api:limiter:task_write"
	ReadTaskRateLimiterKey  RateLimiterKey = "dojo_worker_api:limiter:task_read"
	MetricsRateLimiterKey   RateLimiterKey = "dojo_worker_api:limiter:metrics"
	GeneralRateLimiterKey   RateLimiterKey = "dojo_worker_api:limiter:general"
)

type LimiterConfig struct {
	key    RateLimiterKey
	rate   limiter.Rate
	prefix string
}

func init() {
	InitializeLimiters()
}

func GeneralRateLimiter() gin.HandlerFunc {
	return getRateLimiterMiddleware(GeneralRateLimiterKey)
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

func WorkerRateLimiter() gin.HandlerFunc {
	return getRateLimiterMiddleware(WorkerRateLimiterKey)
}

func InitializeLimiters() {
	once.Do(func() {
		cache := cache.GetCacheInstance()

		limiterConfigs := []LimiterConfig{
			{
				key:    WorkerRateLimiterKey,
				rate:   limiter.Rate{Period: 1 * time.Minute, Limit: 60},
				prefix: string(WorkerRateLimiterKey),
			},
			{
				key:    WriteTaskRateLimiterKey,
				rate:   limiter.Rate{Period: 1 * time.Hour, Limit: 60},
				prefix: string(WriteTaskRateLimiterKey),
			},
			{
				key:    ReadTaskRateLimiterKey,
				rate:   limiter.Rate{Period: 1 * time.Minute, Limit: 60},
				prefix: string(ReadTaskRateLimiterKey),
			},
			{
				key:    MetricsRateLimiterKey,
				rate:   limiter.Rate{Period: 1 * time.Hour, Limit: 1800},
				prefix: string(MetricsRateLimiterKey),
			},
			{
				key:    GeneralRateLimiterKey,
				rate:   limiter.Rate{Period: 1 * time.Hour, Limit: 3600},
				prefix: string(GeneralRateLimiterKey),
			},
		}

		for _, config := range limiterConfigs {
			store, err := sredis.NewStoreWithOptions(&cache.Redis, limiter.StoreOptions{
				Prefix:   config.prefix,
				MaxRetry: 3,
			})
			if err != nil {
				log.Fatal().Err(err).Str("prefix", config.prefix).Msg("Failed to create rate limiter store")
				continue
			}
			rateLimiter := limiter.New(store, config.rate)
			if runtimeEnv := utils.LoadDotEnv("RUNTIME_ENV"); runtimeEnv == "aws" {
				rateLimiter = limiter.New(store, config.rate, limiter.WithClientIPHeader("X-Original-Forwarded-For"))
			}

			limiters.Store(config.key, rateLimiter)
		}
	})
}

func getRateLimiterMiddleware(key RateLimiterKey) gin.HandlerFunc {
	return func(c *gin.Context) {
		limiterInstance, ok := limiters.Load(key)
		if !ok {
			log.Fatal().Str("key", string(key)).Msg("Rate limiters not initialized properly")
			c.Error(errors.New("Internal Server Error"))
			c.AbortWithStatusJSON(500, gin.H{"error": "Internal Server Error"})
			return
		}

		limiter := limiterInstance.(*limiter.Limiter)
		ip := getCallerIP(c)
		log.Debug().Msgf("Rate limiting for IP %s", ip)

		limiterCtx, err := limiter.Get(c, ip)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get rate limiter")
			c.Error(errors.New("Internal Server Error"))
			c.AbortWithStatusJSON(500, gin.H{"error": "Internal Server Error"})
			return
		}

		if limiterCtx.Reached {
			log.Error().Msg("Too Many Requests")
			c.Error(errors.New("Too many requests"))
			c.AbortWithStatusJSON(429, gin.H{"error": "Too Many Requests"})
			return
		}

		c.Next()
	}
}

// Middleware that checks if the caller is in the metagraph and aborts with
// a 403 status if not. This is to prevent random people from being able to
// hit our APIs.
func InMetagraphOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := getCallerIP(c)
		log.Info().Msgf("Checking if IP %s is in the metagraph", ip)
		subnetSubscriber := blockchain.GetSubnetStateSubscriberInstance()
		found := subnetSubscriber.FindMinerIpAddress(ip)
		if !found {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		log.Debug().Msgf("IP %s is in the metagraph", ip)
		c.Next()
	}
}
