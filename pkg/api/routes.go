package api

import (
	"github.com/gin-gonic/gin"
)

func LoginRoutes(router *gin.Engine) {

	// Grouping routes
	workerApiGroup := router.Group("/api/v1")
	{
		workerApiGroup.POST("/login/auth", WorkerLoginMiddleware(), WorkerLoginController)
		workerApiGroup.POST("/tasks/", WorkerAuthMiddleware(), CreateTaskController)
		workerApiGroup.PUT("/tasks/:task-id", SubmitWorkerTaskController)
	}
	minerApiGroup := router.Group("/api/miner")
    {
    	minerApiGroup.POST("/login/auth", MinerLoginMiddleware(), MinerLoginController)
    }
}
