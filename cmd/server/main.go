package main

import (
	"dojo-api/pkg/api"
	"dojo-api/utils"

	"github.com/gin-gonic/gin"
	"github.com/gin-contrib/cors"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal().Msg("Error loading .env file")
	}

	port := utils.LoadDotEnv("SERVER_PORT")

	r := gin.Default()
	r.Use(cors.Default())
	api.LoginRoutes(r)
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Hello, this is dojo-go-api",
		})
	})
	r.Run(":" + port)
}
