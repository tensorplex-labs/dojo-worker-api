package api

import (
	"dojo-api/docs"

	"github.com/gin-gonic/gin"
)

// GET /api/v1/metrics/dojo-worker-count
func LoginRoutes(router *gin.Engine) {
	docs.SwaggerInfo.BasePath = "/api/v1"
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
			tasks.GET("/task-result/:task-id", GetTaskResultsController)
			tasks.GET("/:task-id", GetTaskByIdController)
			tasks.GET("/", WorkerAuthMiddleware(), GetTasksByPageController)
		}

		miner := apiV1.Group("/miner")
		{
			// miner.POST("/login/auth", MinerLoginMiddleware(), MinerLoginController)
			// miner.GET("/info/:hotkey", MinerAuthMiddleware(), MinerInfoController)
			// miner.POST("/miner-application", MinerVerificationMiddleware(), MinerApplicationController)
			// miner.PUT("/partner/disable", MinerAuthMiddleware(), DisableWorkerByMinerController)
			miner.POST("/session/auth", GenerateCookieAuth)

			apiKeyGroup := miner.Group("/api-key")
			{
				apiKeyGroup.GET("/list", MinerCookieAuthMiddleware(), MinerApiKeyListController)
				apiKeyGroup.POST("/generate", MinerCookieAuthMiddleware(), MinerApiKeyGenerateController)
				apiKeyGroup.PUT("/disable", MinerCookieAuthMiddleware(), MinerApiKeyDisableController)
			}

			subScriptionKeyGroup := miner.Group("/subscription-key")
			{
				subScriptionKeyGroup.GET("/list", MinerCookieAuthMiddleware(), MinerSubscriptionKeyListController)
				subScriptionKeyGroup.POST("/generate", MinerCookieAuthMiddleware(), MinerSubscriptionKeyGenerateController)
				subScriptionKeyGroup.PUT("/disable", MinerCookieAuthMiddleware(), MinerSubscriptionKeyDisableController)
			}
		}
		metrics := apiV1.Group("/metrics")
		{
			metrics.GET("/dojo-worker-count", GetDojoWorkerCountController)
			metrics.GET("/completed-tasks-count", GetTotalCompletedTasksController)
			metrics.GET("/task-result-count", GetTotalTasksResultsController)
			metrics.GET("/average-task-completion-time", GetAvgTaskCompletionTimeController)
		}
	}
}
