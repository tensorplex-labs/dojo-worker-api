package api

import (
    "github.com/gin-gonic/gin"
    "dojo-api/pkg/orm"
    "net/http"
)

func LoginController(c *gin.Context) {
    walletAddressInterface, _ := c.Get("WalletAddress")
    signatureInterface, _ := c.Get("Signature")
	token, _ := c.Get("JWTToken")

    walletAddress, ok := walletAddressInterface.(string)
    if !ok {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid wallet address"})
        return
    }
    signature, ok := signatureInterface.(string)
    if !ok {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid signature"})
        return
    }
    
    workerService := orm.NewDojoWorkerService()
    _, err := workerService.CreateDojoWorker(walletAddress, signature)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create worker"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"token": token})
}

