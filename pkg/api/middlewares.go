package api

import (
	"github.com/gin-gonic/gin"
	"dojo-api/db"
	"github.com/rs/zerolog/log"
	"context"
	"dojo-api/utils"
)

func UserAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.Request.Header.Get("X-API-KEY")

		if userId := verifyApiKey(apiKey); userId != "" {
			c.Set("networkUserID", userId)
			c.Next()
		} else {
			c.JSON(401, gin.H{"error": "Unauthorized"})
			c.Abort()
		}
	}
}

func verifyApiKey(apiKey string) string{
	// Check if the API key is valid
	client := db.NewClient()
	ctx := context.Background()
	logger := utils.GetLogger()
	defer func() {
        if err := client.Prisma.Disconnect(); err != nil {
            logger.Error().Msgf("Error disconnecting: %v", err)
        }
    }()
	client.Prisma.Connect()
    apiKeyModel, err := client.NetworkUser.FindFirst(
        db.NetworkUser.APIKey.Equals(apiKey),
    ).Exec(ctx)

	if err != nil {
		log.Error().Msgf("Error finding API key: %v", err)
		return ""
    }

	if apiKeyModel == nil {
		return ""
	}

	return apiKeyModel.ID
}