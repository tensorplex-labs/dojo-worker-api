package orm

import (
	"context"
	"errors"

	"dojo-api/db"

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

	return apiKeys, nil
}

func (a *SubscriptionKeyORM) CreateSubscriptionKeyByHotkey(hotkey string, subscriptionKey string) (*db.SubscriptionKeyModel, error) {
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

	return foundSubscriptionKey, nil
}
