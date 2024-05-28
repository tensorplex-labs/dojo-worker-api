package orm

import (
	"context"
	"dojo-api/db"

	"github.com/rs/zerolog/log"
)

type ApiKeyORM struct {
	dbClient      *db.PrismaClient
	clientWrapper *PrismaClientWrapper
}

func NewApiKeyORM() *ApiKeyORM {
	clientWrapper := GetPrismaClient()
	return &ApiKeyORM{dbClient: clientWrapper.Client, clientWrapper: clientWrapper}
}

func (a *ApiKeyORM) GetApiKeysByMinerHotkey(hotkey string) ([]db.APIKeyModel, error) {
	a.clientWrapper.BeforeQuery()
	defer a.clientWrapper.AfterQuery()

	ctx := context.Background()

	minerUser, err := NewMinerUserORM().GetUserByHotkey(hotkey)
	if err != nil {
		log.Error().Err(err).Msgf("Error getting miner user")
		return nil, err
	}

	apiKeys, err := a.dbClient.APIKey.FindMany(
		db.APIKey.MinerUserID.Equals(minerUser.ID),
		db.APIKey.IsDelete.Equals(false),
	).Exec(ctx)
	if err != nil {
		log.Error().Err(err).Msgf("Error getting api keys")
		return nil, err
	}

	return apiKeys, nil
}

func (a *ApiKeyORM) CreateApiKeyByHotkey(hotkey string, apiKey string) (*db.APIKeyModel, error) {
	a.clientWrapper.BeforeQuery()
	defer a.clientWrapper.AfterQuery()

	ctx := context.Background()

	minerUser, err := NewMinerUserORM().GetUserByHotkey(hotkey)
	if err != nil {
		log.Error().Err(err).Msgf("Error getting miner user")
		return nil, err
	}

	createdApiKey, err := a.dbClient.APIKey.CreateOne(
		db.APIKey.Key.Set(apiKey),
		db.APIKey.MinerUser.Link(
			db.MinerUser.ID.Equals(minerUser.ID),
		),
		db.APIKey.IsDelete.Set(false),
	).Exec(ctx)

	if err != nil {
		log.Error().Err(err).Msgf("Error creating api key")
		return nil, err
	}
	return createdApiKey, nil
}

func (a *ApiKeyORM) DisableApiKeyByHotkey(hotkey string, apiKey string) (*db.APIKeyModel, error) {
	a.clientWrapper.BeforeQuery()
	defer a.clientWrapper.AfterQuery()

	ctx := context.Background()
	disabledApiKey, err := a.dbClient.APIKey.FindUnique(
		db.APIKey.Key.Equals(apiKey),
	).Update(
		db.APIKey.IsDelete.Set(true),
	).Exec(ctx)

	if err != nil {
		log.Error().Err(err).Msgf("Error disabling api key")
		return nil, err
	}
	return disabledApiKey, nil
}
