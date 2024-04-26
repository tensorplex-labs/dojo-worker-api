package orm

import (
	"context"
	"dojo-api/db"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

type NetworkUserORM struct {
	dbClient *db.PrismaClient
}

func NewNetworkUserService() *NetworkUserORM {
	client := NewPrismaClient()
	return &NetworkUserORM{
		dbClient: client,
	}
}

func (s *NetworkUserORM) CreateUser() (*db.MinerUserModel, error) {
	ctx := context.Background()
	createdUser, err := s.dbClient.MinerUser.CreateOne(
		db.MinerUser.Coldkey.Set("asd123"),
		db.MinerUser.Hotkey.Set("asd123"),
		db.MinerUser.APIKey.Set(""),
		db.MinerUser.APIKeyExpireAt.Set(time.Now().Add(time.Hour*24*7)),
		db.MinerUser.IsVerified.Set(false),
	).Exec(ctx)

	if err != nil {
		log.Error().Err(err).Msg("Failed to create user")
		return nil, err
	}

	log.Info().Str("minerUser", fmt.Sprintf("%+v", createdUser)).Msg("User created successfully")
	return createdUser, nil
}
