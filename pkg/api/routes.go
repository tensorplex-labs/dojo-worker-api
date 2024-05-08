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
			worker.POST("/partner", WorkerAuthMiddleware(), WorkerPartnerCreateController)
			worker.PUT("/partner/disable", WorkerAuthMiddleware(), DisableMinerByWorkerController)
			worker.GET("/partner/list", WorkerAuthMiddleware(), GetWorkerPartnerListController)
		}
		apiV1.GET("/auth/:address", GenerateNonceController)
		apiV1.PUT("/partner/edit", WorkerAuthMiddleware(), UpdateWorkerPartnerController)
		tasks := apiV1.Group("/tasks")
		{
			tasks.POST("/create-tasks", MinerAuthMiddleware(), CreateTasksController)
			tasks.PUT("/submit-result/:task-id", WorkerAuthMiddleware(), SubmitTaskResultController)
			tasks.GET("/:task-id", GetTaskByIdController)
			tasks.GET("/", WorkerAuthMiddleware(), GetTasksByPageController)
		}

		miner := apiV1.Group("/miner")
		{
			// miner.POST("/login/auth", MinerLoginMiddleware(), MinerLoginController)
			miner.GET("/info/:hotkey", MinerAuthMiddleware(), MinerInfoController)
			miner.POST("/miner-application", MinerVerificationMiddleware(), MinerApplicationController)
			miner.PUT("/partner/disable", MinerAuthMiddleware(), DisableWorkerByMinerController)
		}
	}
}
