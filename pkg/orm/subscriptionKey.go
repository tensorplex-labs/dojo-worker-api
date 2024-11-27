package orm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"dojo-api/db"
	"dojo-api/pkg/cache"

	"github.com/rs/zerolog/log"
)

type SubscriptionKeyORM struct {
	dbClient      *db.PrismaClient
	clientWrapper *PrismaClientWrapper
}

type SubscriptionKeyCacheKey string

const (
	SubKeysByHotkeyCacheKey SubscriptionKeyCacheKey = "sk_by_hotkey"
	SubKeyByKeyCacheKey     SubscriptionKeyCacheKey = "sk_by_key"
)

type SubscriptionKeyCache struct {
	key    SubscriptionKeyCacheKey
	hotkey string
	subKey string
}

func NewSubscriptionKeyCache(key SubscriptionKeyCacheKey) *SubscriptionKeyCache {
	return &SubscriptionKeyCache{
		key: key,
	}
}

func (sc *SubscriptionKeyCache) GetCacheKey() string {
	switch sc.key {
	case SubKeysByHotkeyCacheKey:
		return fmt.Sprintf("%s:%s", sc.key, sc.hotkey)
	case SubKeyByKeyCacheKey:
		return fmt.Sprintf("%s:%s", sc.key, sc.subKey)
	default:
		return fmt.Sprintf("sk:%s", sc.subKey)
	}
}

func (sc *SubscriptionKeyCache) GetExpiration() time.Duration {
	return 5 * time.Minute
}

func NewSubscriptionKeyORM() *SubscriptionKeyORM {
	clientWrapper := GetPrismaClient()
	return &SubscriptionKeyORM{dbClient: clientWrapper.Client, clientWrapper: clientWrapper}
}

func (a *SubscriptionKeyORM) GetSubscriptionKeysByMinerHotkey(hotkey string) ([]db.SubscriptionKeyModel, error) {
	subCache := NewSubscriptionKeyCache(SubKeysByHotkeyCacheKey)
	subCache.hotkey = hotkey

	var subKeys []db.SubscriptionKeyModel
	cache := cache.GetCacheInstance()

	// Try to get from cache first
	if err := cache.GetCache(subCache, &subKeys); err == nil {
		return subKeys, nil
	}

	ctx := context.Background()

	minerUser, err := NewMinerUserORM().GetUserByHotkey(hotkey)
	if err != nil {
		log.Error().Err(err).Msgf("Error getting miner user")
		return nil, err
	}

	apiKeys, err := a.dbClient.SubscriptionKey.FindMany(
		db.SubscriptionKey.And(
			db.SubscriptionKey.MinerUserID.Equals(minerUser.ID),
			db.SubscriptionKey.IsDelete.Equals(false),
		),
	).Exec(ctx)
	if err != nil {
		log.Error().Err(err).Msgf("Error getting subscription keys")
		return nil, err
	}

	// Cache the result
	if err := cache.SetCache(subCache, apiKeys); err != nil {
		log.Error().Err(err).Msgf("Error caching subscription keys")
	}

	return apiKeys, nil
}

func (a *SubscriptionKeyORM) CreateSubscriptionKeyByHotkey(hotkey string, subscriptionKey string) (*db.SubscriptionKeyModel, error) {
	a.clientWrapper.BeforeQuery()
	defer a.clientWrapper.AfterQuery()

	ctx := context.Background()

	minerUser, err := NewMinerUserORM().GetUserByHotkey(hotkey)
	if err != nil {
		log.Error().Err(err).Msgf("Error getting miner user")
		return nil, err
	}

	createdApiKey, err := a.dbClient.SubscriptionKey.CreateOne(
		db.SubscriptionKey.Key.Set(subscriptionKey),
		db.SubscriptionKey.MinerUser.Link(
			db.MinerUser.ID.Equals(minerUser.ID),
		),
		db.SubscriptionKey.IsDelete.Set(false),
	).Exec(ctx)
	if err != nil {
		log.Error().Err(err).Msgf("Error creating subscription key")
		return nil, err
	}
	return createdApiKey, nil
}

func (a *SubscriptionKeyORM) DisableSubscriptionKeyByHotkey(hotkey string, subscriptionKey string) (*db.SubscriptionKeyModel, error) {
	a.clientWrapper.BeforeQuery()
	defer a.clientWrapper.AfterQuery()

	ctx := context.Background()
	disabledAPIKey, err := a.dbClient.SubscriptionKey.FindUnique(
		db.SubscriptionKey.Key.Equals(subscriptionKey),
	).Update(
		db.SubscriptionKey.IsDelete.Set(true),
	).Exec(ctx)
	if err != nil {
		log.Error().Err(err).Msgf("Error disabling subscription key")
		return nil, err
	}
	return disabledAPIKey, nil
}

func (a *SubscriptionKeyORM) GetSubscriptionByKey(subScriptionKey string) (*db.SubscriptionKeyModel, error) {
	subCache := NewSubscriptionKeyCache(SubKeyByKeyCacheKey)
	subCache.subKey = subScriptionKey

	var foundSubscriptionKey *db.SubscriptionKeyModel
	cache := cache.GetCacheInstance()

	// Try to get from cache first
	if err := cache.GetCache(subCache, &foundSubscriptionKey); err == nil {
		return foundSubscriptionKey, nil
	}
	a.clientWrapper.BeforeQuery()
	defer a.clientWrapper.AfterQuery()

	ctx := context.Background()

	foundSubscriptionKey, err := a.dbClient.SubscriptionKey.FindFirst(
		db.SubscriptionKey.Key.Equals(subScriptionKey),
	).With(
		db.SubscriptionKey.MinerUser.Fetch(),
	).Exec(ctx)
	if err != nil {
		if db.IsErrNotFound(err) {
			log.Error().Err(err).Msgf("Subscription key not found")
			return nil, errors.New("subscription key not found")
		}
		log.Error().Err(err).Msgf("Error getting Subscription key")
		return nil, err
	}

	// Cache the result
	if err := cache.SetCache(subCache, foundSubscriptionKey); err != nil {
		log.Error().Err(err).Msgf("Error caching subscription key")
	}

	return foundSubscriptionKey, nil
}
