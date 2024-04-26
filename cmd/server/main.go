package main

import (
    "github.com/gin-gonic/gin"
    "github.com/joho/godotenv"
    "dojo-api/pkg/api"
    "log"
    "os"
)

func main() {
    err := godotenv.Load()
    if err != nil {
        log.Fatal("Error loading .env file")
    }

    port := os.Getenv("SERVER_PORT")
    if port == "" {
        port = "8080" // Default port if not specified
    }

    r := gin.Default()
    api.LoginRoutes(r)
    r.GET("/", func(c *gin.Context) {
        c.JSON(200, gin.H{
            "message": "Hello, this is dojo-go-api",
        })
    })
    r.Run(":" + port)
}