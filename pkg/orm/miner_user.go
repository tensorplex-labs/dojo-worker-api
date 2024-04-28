package orm

import (
	"context"
	"dojo-api/db"
	"fmt"
	"log"
	"time"
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

func (s *MinerUserORM) CreateUser(coldkey string, hotkey string, apiKey string, expiry time.Time, isVerified bool) (*db.MinerUserModel, error) {
	ctx := context.Background()
	createdUser, err := s.dbClient.MinerUser.CreateOne(
		db.MinerUser.Coldkey.Set(coldkey),
		db.MinerUser.Hotkey.Set(hotkey),
		db.MinerUser.APIKey.Set(apiKey),
		db.MinerUser.APIKeyExpireAt.Set(expiry),
		db.MinerUser.IsVerified.Set(isVerified),
	).Exec(ctx)

	if err != nil {
		log.Printf("Error creating user: %v", err)
		return nil, err
	}
	log.Println("User created successfully")
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
		log.Printf("Error retrieving user by API key: %v", err)
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
		log.Printf("Error retrieving user by hotkey: %v", err)
		return nil, err
	}
	return user, nil
}
