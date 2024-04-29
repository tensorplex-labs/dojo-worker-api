package main

import (
	"dojo-api/pkg/auth"
	"dojo-api/pkg/orm"
	"fmt"
	"time"
)

func main() {
	// TODO remove if not necessary
	fmt.Println("Hello, World!")
	authClient := auth.NewAuthService()
	fmt.Println(authClient)
	minerUserORM := orm.NewMinerUserORM()
	minerUserORM.CreateUser("coldkey123", "hotkey123", "apiKey123", time.Now().Add(24*time.Hour), true)
}
