package orm

import (
	"context"
	"fmt"
	"time"

	"dojo-api/db"

	"github.com/rs/zerolog/log"
)

type MinerUserORM struct {
	dbClient *db.PrismaClient
}

func NewMinerUserORM() *MinerUserORM {
	client := NewPrismaClient()
	return &MinerUserORM{
		dbClient: client,
	}
}

func (s *MinerUserORM) CreateUserWithOrganisation(hotkey string, apiKey string, expiry time.Time, isVerified bool, email string, organisation string) (*db.MinerUserModel, error) {
	ctx := context.Background()
	createdUser, err := s.dbClient.MinerUser.CreateOne(
		db.MinerUser.Hotkey.Set(hotkey),
		db.MinerUser.APIKey.Set(apiKey),
		db.MinerUser.APIKeyExpireAt.Set(expiry),
		db.MinerUser.Email.Set(email),
		db.MinerUser.OrganizationName.Set(organisation),
		db.MinerUser.IsVerified.Set(isVerified),
	).Exec(ctx)
	if err != nil {
		log.Error().Err(err).Msgf("Error creating user")
		return nil, err
	}
	log.Info().Msg("User created successfully")
	return createdUser, nil
}

func (s *MinerUserORM) CreateUser(hotkey string, apiKey string, expiry time.Time, isVerified bool, email string) (*db.MinerUserModel, error) {
	ctx := context.Background()
	createdUser, err := s.dbClient.MinerUser.CreateOne(
		db.MinerUser.Hotkey.Set(hotkey),
		db.MinerUser.APIKey.Set(apiKey),
		db.MinerUser.APIKeyExpireAt.Set(expiry),
		db.MinerUser.Email.Set(email),
		db.MinerUser.IsVerified.Set(isVerified),
	).Exec(ctx)
	if err != nil {
		log.Error().Err(err).Msgf("Error creating user")
		return nil, err
	}
	log.Info().Msg("User created successfully")
	return createdUser, nil
}

func (s *MinerUserORM) GetUserByAPIKey(apiKey string) (*db.MinerUserModel, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key cannot be an empty string")
	}
	ctx := context.Background()
	user, err := s.dbClient.MinerUser.FindFirst(
		db.MinerUser.APIKey.Equals(apiKey),
	).Exec(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Error retrieving user by API key")
		return nil, err
	}
	return user, nil
}

func (s *MinerUserORM) GetUserByHotkey(hotkey string) (*db.MinerUserModel, error) {
	if hotkey == "" {
		return nil, fmt.Errorf("hotkey cannot be an empty string")
	}
	ctx := context.Background()
	user, err := s.dbClient.MinerUser.FindFirst(
		db.MinerUser.Hotkey.Equals(hotkey),
	).Exec(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Error retrieving user by hotkey")
		return nil, err
	}
	return user, nil
}

func (s *MinerUserORM) SetVerified(id string) error {
	if id == "" {
		return fmt.Errorf("id cannot be an empty string")
	}
	ctx := context.Background()
	_, err := s.dbClient.MinerUser.FindUnique(
		db.MinerUser.ID.Equals(id),
	).Update(
		db.MinerUser.IsVerified.Set(true),
	).Exec(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Error setting user as verified")
		return err
	}
	return nil
}
