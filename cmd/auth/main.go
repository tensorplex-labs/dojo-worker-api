package main

import (
	"dojo-api/pkg/auth"
	"dojo-api/pkg/orm"
	"fmt"
)

func main() {
	// TODO remove if not necessary
	fmt.Println("Hello, World!")
	authClient := auth.NewAuthService()
	fmt.Println(authClient)
	service := orm.NewNetworkUserService()
	service.CreateUser()
}
