package api

import (
	"github.com/gin-gonic/gin"
)

func LoginRoutes(router *gin.Engine) {
	apiV1 := router.Group("/api/v1")
	{
		worker := apiV1.Group("/worker")
		{
			worker.POST("/login/auth", WorkerLoginMiddleware(), WorkerLoginController)
			// TODO verify that worker is logged in using WorkerAuthMiddleware
			worker.POST("/partner", WorkerAuthMiddleware(), WorkerPartnerController)
		}
		apiV1.POST("/tasks/create-task", MinerLoginMiddleware(), CreateTaskController)
		// TODO verify that worker is logged in using WorkerAuthMiddleware
		apiV1.PUT("/tasks/submit-result/:task-id", WorkerAuthMiddleware(), SubmitTaskResultController)

		miner := apiV1.Group("/miner")
		{
			miner.POST("/login/auth", MinerLoginMiddleware(), MinerLoginController)
			miner.GET(":hotkey", MinerController)
		}
	}
}
