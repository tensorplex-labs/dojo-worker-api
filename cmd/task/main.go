package main

import (
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	port := os.Getenv("PORT")
	fmt.Println("Using port:", port)
	if port == "" {
		port = "4001" // default to 4001 if no environment variable is set
	}
	port = ":" + port

	router := gin.Default()

	// Hello World
	router.GET("/hello-world", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Hello World",
		})
	})

	router.Run(port) // Default listens on :8080
}
