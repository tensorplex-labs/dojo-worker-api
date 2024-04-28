package orm

import (
	"context"
	"dojo-api/db"
	"errors"

	"github.com/rs/zerolog/log"
)

type MinerUserORM struct {
	dbClient *db.PrismaClient
}

func NewMinerUserORM() *MinerUserORM {
	client := NewPrismaClient()
	return &MinerUserORM{dbClient: client}
}

func (m *MinerUserORM) GetByApiKey(apiKey string) (*db.MinerUserModel, error) {
	ctx := context.Background()
	minerUser, err := m.dbClient.MinerUser.FindFirst(
		db.MinerUser.APIKey.Equals(apiKey),
	).Exec(ctx)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			log.Error().Err(err).Str("apiKey", apiKey).Msgf("Miner user not found with API key")
			return nil, err
		}
		return nil, err
	}

	return minerUser, nil
}
