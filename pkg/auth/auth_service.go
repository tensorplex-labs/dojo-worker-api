package auth

import (
	"dojo-api/db"
	"dojo-api/pkg/orm"
)

type AuthService struct {
	client *db.PrismaClient
}

func NewAuthService() *AuthService {
	return &AuthService{
		client: orm.NewPrismaClient(),
	}
}
