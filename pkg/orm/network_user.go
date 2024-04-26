package orm

import (
	"dojo-api/db"
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

// func (s *NetworkUserORM) CreateUser() (*db.NetworkUserModel, error) {
// 	ctx := context.Background()
// 	// TODO add actual logic
// 	createdUser, err := s.dbClient.NetworkUser.CreateOne(
// 		db.NetworkUser.Coldkey.Set("asd123"),
// 		db.NetworkUser.Hotkey.Set("asd123"),
// 		db.NetworkUser.APIKey.Set(""),
// 		db.NetworkUser.KeyExpireAt.Set(time.Now().Add(time.Hour*24*7)),
// 		db.NetworkUser.IPAddress.Set("asdasd"),
// 		db.NetworkUser.UserType.Set(db.NetworkUserTypeValidator),
// 		db.NetworkUser.IsVerified.Set(false),
// 	).Exec(ctx)

// 	if err != nil {
// 		log.Printf("Error creating user: %v", err)
// 		return nil, err
// 	}
// 	log.Println("User created successfully")
// 	return createdUser, nil
// }
