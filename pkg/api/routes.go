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

		tasks := apiV1.Group("/tasks")
		{
			tasks.POST("/create-task", MinerLoginMiddleware(), CreateTaskController)
			tasks.PUT("/submit-result/:task-id", WorkerAuthMiddleware(), SubmitTaskResultController)
			tasks.GET("/:task-id", GetTaskByIdController)
			tasks.GET("/", GetTasksByPageController)
		}

		miner := apiV1.Group("/miner")
		{
			miner.POST("/login/auth", MinerLoginMiddleware(), MinerLoginController)
			miner.GET(":hotkey", MinerController)
		}
	}
}
