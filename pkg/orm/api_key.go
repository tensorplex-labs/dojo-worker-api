package orm

import (
	"context"
	"errors"

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

func (a *ApiKeyORM) GetByApiKey(apiKey string) (*db.APIKeyModel, error) {
	a.clientWrapper.BeforeQuery()
	defer a.clientWrapper.AfterQuery()

	ctx := context.Background()

	foundApiKey, err := a.dbClient.APIKey.FindFirst(
		db.APIKey.Key.Equals(apiKey),
	).With(
		db.APIKey.MinerUser.Fetch(),
	).Exec(ctx)
	if err != nil {
		if db.IsErrNotFound(err) {
			log.Error().Err(err).Msgf("API key not found")
			return nil, errors.New("API key not found")
		}
		log.Error().Err(err).Msgf("Error getting api key")
		return nil, err
	}

	return foundApiKey, nil
}
