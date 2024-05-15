package orm

import (
	"context"
	"fmt"
	"time"

	"dojo-api/db"

	"github.com/rs/zerolog/log"
)

type MinerUserORM struct {
	dbClient      *db.PrismaClient
	clientWrapper *PrismaClientWrapper
}

func NewMinerUserORM() *MinerUserORM {
	clientWrapper := GetPrismaClient()
	return &MinerUserORM{
		dbClient:      clientWrapper.Client,
		clientWrapper: clientWrapper,
	}
}

func (s *MinerUserORM) CreateUserWithOrganisation(hotkey string, apiKey string, expiry time.Time, isVerified bool, email string, subscriptionKey string, organisation string) (*db.MinerUserModel, error) {
	s.clientWrapper.BeforeQuery()
	defer s.clientWrapper.AfterQuery()

	ctx := context.Background()
	createdUser, err := s.dbClient.MinerUser.CreateOne(
		db.MinerUser.Hotkey.Set(hotkey),
		db.MinerUser.APIKey.Set(apiKey),
		db.MinerUser.APIKeyExpireAt.Set(expiry),
		db.MinerUser.SubscriptionKey.Set(subscriptionKey),
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

func (s *MinerUserORM) SetVerified(isVerified bool, id string) (*db.MinerUserModel, error) {
	s.clientWrapper.BeforeQuery()
	defer s.clientWrapper.AfterQuery()

	ctx := context.Background()
	updatedUser, err := s.dbClient.MinerUser.FindUnique(
		db.MinerUser.ID.Equals(id),
	).Update(
		db.MinerUser.IsVerified.Set(isVerified),
	).Exec(ctx)

	if err != nil {
		log.Error().Err(err).Msgf("Error creating user")
		return nil, err
	}
	log.Info().Msg("User created successfully")
	return updatedUser, nil
}

func (s *MinerUserORM) CreateUser(hotkey string, apiKey string, expiry time.Time, isVerified bool, email string, subscriptionKey string) (*db.MinerUserModel, error) {
	s.clientWrapper.BeforeQuery()
	defer s.clientWrapper.AfterQuery()

	ctx := context.Background()
	createdUser, err := s.dbClient.MinerUser.CreateOne(
		db.MinerUser.Hotkey.Set(hotkey),
		db.MinerUser.APIKey.Set(apiKey),
		db.MinerUser.APIKeyExpireAt.Set(expiry),
		db.MinerUser.SubscriptionKey.Set(subscriptionKey),
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
	s.clientWrapper.BeforeQuery()
	defer s.clientWrapper.AfterQuery()

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
	s.clientWrapper.BeforeQuery()
	defer s.clientWrapper.AfterQuery()
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

func (s *MinerUserORM) GetUserBySubscriptionKey(subscriptionKey string) (*db.MinerUserModel, error) {
	s.clientWrapper.BeforeQuery()
	defer s.clientWrapper.AfterQuery()

	ctx := context.Background()
	user, err := s.dbClient.MinerUser.FindFirst(
		db.MinerUser.SubscriptionKey.Equals(subscriptionKey),
	).Exec(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Error retrieving user by subscription key")
		return nil, err
	}
	return user, nil
}

func (s *MinerUserORM) DeregisterMiner(hotkey string) error {
	s.clientWrapper.BeforeQuery()
	defer s.clientWrapper.AfterQuery()

	ctx := context.Background()
	_, err := s.dbClient.MinerUser.FindUnique(
		db.MinerUser.Hotkey.Equals(hotkey),
	).Update(
		db.MinerUser.IsVerified.Set(false),
		db.MinerUser.APIKeyExpireAt.Set(time.Now()),
	).Exec(ctx)

	if err != nil {
		if err == db.ErrNotFound {
			log.Info().Msg("User not found, continuing...")
			return nil
		}
		log.Error().Err(err).Msg("Error deregistering user")
		return err
	}
	log.Info().Msg("Miner deregistered successfully")
	return nil
}

func (s *MinerUserORM) ReregisterMiner(hotkey string) error {
	ctx := context.Background()
	_, err := s.dbClient.MinerUser.FindUnique(
		db.MinerUser.Hotkey.Equals(hotkey),
	).Update(
		db.MinerUser.IsVerified.Set(true),
		db.MinerUser.APIKeyExpireAt.Set(time.Now().Add(time.Hour * 24)),
	).Exec(ctx)

	if err != nil {
		if err == db.ErrNotFound {
			log.Info().Msg("User not found, continuing...")
			return nil
		}
		log.Error().Err(err).Msg("Error reregistering user")
		return err
	}
	log.Info().Msg("Miner reregistered successfully")
	return nil
}
