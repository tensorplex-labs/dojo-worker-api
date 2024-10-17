package api

import (
	"dojo-api/docs"

	"github.com/gin-gonic/gin"
)

func LoginRoutes(router *gin.Engine) {
	docs.SwaggerInfo.BasePath = "/api/v1"
	apiV1 := router.Group("/api/v1")
	{
		worker := apiV1.Group("/worker")
		{
			worker.Use(GenerousRateLimiter())
			worker.POST("/login/auth", WorkerLoginMiddleware(), WorkerLoginController)
			worker.POST("/partner", WorkerAuthMiddleware(), WorkerPartnerCreateController)
			worker.PUT("/partner/disable", WorkerAuthMiddleware(), DisableMinerByWorkerController)
			worker.GET("/partner/list", WorkerAuthMiddleware(), GetWorkerPartnerListController)
		}
		apiV1.GET("/auth/:address", GenerousRateLimiter(), GenerateNonceController)
		apiV1.PUT("/partner/edit", GenerousRateLimiter(), WorkerAuthMiddleware(), UpdateWorkerPartnerController)
		tasks := apiV1.Group("/tasks")
		{
			tasks.POST("/create-tasks", WriteTaskRateLimiter(), MinerAuthMiddleware(), CreateTasksController)
			tasks.PUT("/submit-result/:task-id", WriteTaskRateLimiter(), WorkerAuthMiddleware(), SubmitTaskResultController)
			tasks.GET("/task-result/:task-id", ReadTaskRateLimiter(), GetTaskResultsController)
			tasks.GET("/:task-id", ReadTaskRateLimiter(), GetTaskByIdController)
			tasks.GET("/next-task/:task-id", ReadTaskRateLimiter(), GetNextInProgressTaskController)
			tasks.GET("/", ReadTaskRateLimiter(), WorkerAuthMiddleware(), GetTasksByPageController)
		}

		miner := apiV1.Group("/miner")
		{
			miner.POST("/session/auth", GenerousRateLimiter(), GenerateCookieAuth)

			apiKeyGroup := miner.Group("/api-key")
			apiKeyGroup.Use(GenerousRateLimiter())
			{
				apiKeyGroup.GET("/list", MinerCookieAuthMiddleware(), MinerApiKeyListController)
				apiKeyGroup.POST("/generate", MinerCookieAuthMiddleware(), MinerApiKeyGenerateController)
				apiKeyGroup.PUT("/disable", MinerCookieAuthMiddleware(), MinerApiKeyDisableController)
			}

			subScriptionKeyGroup := miner.Group("/subscription-key")
			subScriptionKeyGroup.Use(GenerousRateLimiter())
			{
				subScriptionKeyGroup.GET("/list", MinerCookieAuthMiddleware(), MinerSubscriptionKeyListController)
				subScriptionKeyGroup.POST("/generate", MinerCookieAuthMiddleware(), MinerSubscriptionKeyGenerateController)
				subScriptionKeyGroup.PUT("/disable", MinerCookieAuthMiddleware(), MinerSubscriptionKeyDisableController)
			}
		}
		metrics := apiV1.Group("/metrics")
		{
			metrics.Use(MetricsRateLimiter())
			metrics.GET("/dojo-worker-count", GetDojoWorkerCountController)
			metrics.GET("/completed-tasks-count", GetTotalCompletedTasksController)
			metrics.GET("/task-result-count", GetTotalTasksResultsController)
			metrics.GET("/average-task-completion-time", GetAvgTaskCompletionTimeController)
		}
	}
}
