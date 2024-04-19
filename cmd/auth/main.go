package main

import (
	"dojo-api/pkg/auth"
	"fmt"
)

func main() {
	fmt.Println("Hello, World!")
	authClient := auth.NewAuthClient()
	fmt.Println(authClient)
}
