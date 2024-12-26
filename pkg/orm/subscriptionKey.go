package orm

import (
	"context"
	"errors"

	"dojo-api/db"
	"dojo-api/pkg/cache"

	"github.com/rs/zerolog/log"
)

type SubscriptionKeyORM struct {
	dbClient      *db.PrismaClient
	clientWrapper *PrismaClientWrapper
}

func NewSubscriptionKeyORM() *SubscriptionKeyORM {
	clientWrapper := GetPrismaClient()
	return &SubscriptionKeyORM{dbClient: clientWrapper.Client, clientWrapper: clientWrapper}
}

func (a *SubscriptionKeyORM) GetSubscriptionKeysByMinerHotkey(hotkey string) ([]db.SubscriptionKeyModel, error) {
	var subKeys []db.SubscriptionKeyModel
	cache := cache.GetCacheInstance()
	cacheKey := cache.BuildCacheKey(cache.Keys.SubByHotkey, hotkey)

	// Try to get from cache first
	if err := cache.GetCacheValue(cacheKey, &subKeys); err == nil {
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
	if err := cache.SetCacheValue(cacheKey, apiKeys); err != nil {
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

	createdSubKey, err := a.dbClient.SubscriptionKey.CreateOne(
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
	return createdSubKey, nil
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
	var foundSubscriptionKey *db.SubscriptionKeyModel
	cache := cache.GetCacheInstance()
	cacheKey := cache.BuildCacheKey(cache.Keys.SubByKey, subScriptionKey)

	// Try to get from cache first
	if err := cache.GetCacheValue(cacheKey, &foundSubscriptionKey); err == nil {
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
	if err := cache.SetCacheValue(cacheKey, foundSubscriptionKey); err != nil {
		log.Error().Err(err).Msgf("Error caching subscription key")
	}

	return foundSubscriptionKey, nil
}
