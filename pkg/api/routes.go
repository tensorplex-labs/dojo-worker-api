package api

import (
	"github.com/gin-gonic/gin"
)

func LoginRoutes(router *gin.Engine) {

	// Grouping routes
	workerApiGroup := router.Group("/api/worker")
    {
    	workerApiGroup.POST("/login/auth", WorkerLoginMiddleware(), WorkerLoginController)
    }
	minerApiGroup := router.Group("/api/miner")
    {
    	minerApiGroup.POST("/login/auth", MinerLoginMiddleware(), MinerLoginController)
    }
}
 
