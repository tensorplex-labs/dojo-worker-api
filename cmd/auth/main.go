package main

import (
	"dojo-api/pkg/auth"
	"fmt"
)

func main() {
	// TODO remove if not necessary
	fmt.Println("Hello, World!")
	authClient := auth.NewAuthService()
	fmt.Println(authClient)
}
