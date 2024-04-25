package api

import (
    "github.com/gin-gonic/gin"
    "dojo-api/pkg/orm"
    "net/http"
    "github.com/rs/zerolog/log"
)

func LoginController(c *gin.Context) {
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