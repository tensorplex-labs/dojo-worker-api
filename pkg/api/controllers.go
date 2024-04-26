package api

import (
	"dojo-api/pkg/orm"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func WorkerLoginController(c *gin.Context) {
	walletAddressInterface, _ := c.Get("WalletAddress")
	chainIdInterface, _ := c.Get("ChainId")
	token, _ := c.Get("JWTToken")

	walletAddress, ok := walletAddressInterface.(string)
	if !ok {
		log.Error().Msg("Invalid wallet address provided")
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Invalid wallet address"))
		return
	}
	chainId, ok := chainIdInterface.(string)
	if !ok {
		log.Error().Msg("Invalid signature provided")
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Invalid signature"))
		return
	}

	workerService := orm.NewDojoWorkerService()
	_, err := workerService.CreateDojoWorker(walletAddress, chainId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create worker")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to create worker"))
		return
	}
	log.Info().Str("walletAddress", walletAddress).Msg("Worker created successfully")
	c.JSON(http.StatusOK, defaultSuccessResponse(token))
}

func MinerLoginController(c *gin.Context) {
	verified, _ := c.Get("verified")
	hotkey, _ := c.Get("hotkey")
	coldkey, _ := c.Get("coldkey")
	apiKey, _ := c.Get("apiKey")
	expiry, _ := c.Get("expiry")

	networkUserService := orm.NewMinerUserService()
	_, err := networkUserService.CreateUser(coldkey.(string), hotkey.(string), apiKey.(string), expiry.(time.Time), verified.(bool))
	if err != nil {
		log.Error().Err(err).Msg("Failed to save network user")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to save network user"))
		return
	}

	if verified.(bool) {
		c.JSON(http.StatusOK, defaultSuccessResponse(apiKey))
	} else {
		c.JSON(http.StatusUnauthorized, defaultErrorResponse("Miner user not verified"))
	}
}
