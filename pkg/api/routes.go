package api

import (
	"dojo-api/docs"

	"github.com/gin-gonic/gin"
)

func LoginRoutes(router *gin.Engine) {
	docs.SwaggerInfo.BasePath = "/api/v1"
	apiV1 := router.Group("/api/v1")
	apiV1.Use(ResourceProfiler())
	{
		worker := apiV1.Group("/worker")
		{
			worker.Use(WorkerRateLimiter())
			worker.POST("/login/auth", WorkerLoginMiddleware(), WorkerLoginController)
			worker.POST("/partner", WorkerAuthMiddleware(), WorkerPartnerCreateController)
			worker.PUT("/partner/disable", WorkerAuthMiddleware(), DisableMinerByWorkerController)
			worker.GET("/partner/list", WorkerAuthMiddleware(), GetWorkerPartnerListController)
		}
		apiV1.GET("/auth/:address", GeneralRateLimiter(), GenerateNonceController)
		apiV1.PUT("/partner/edit", GeneralRateLimiter(), WorkerAuthMiddleware(), UpdateWorkerPartnerController)
		tasks := apiV1.Group("/tasks")
		{
			tasks.PUT("/submit-result/:task-id", WorkerAuthMiddleware(), SubmitTaskResultController)
			// TODO: re-enable InMetagraphOnly(), and rate limiter in future
			tasks.POST("/create-tasks", MinerAuthMiddleware(), CreateTasksController)
			tasks.GET("/task-result/:task-id", ReadTaskRateLimiter(), GetTaskResultsController)
			tasks.GET("/:task-id", ReadTaskRateLimiter(), GetTaskByIdController)
			tasks.GET("/next-task/:task-id", ReadTaskRateLimiter(), WorkerAuthMiddleware(), GetNextInProgressTaskController)
			tasks.GET("/", ReadTaskRateLimiter(), WorkerAuthMiddleware(), GetTasksByPageController)
		}

		miner := apiV1.Group("/miner")
		{
			miner.POST("/session/auth", GeneralRateLimiter(), GenerateCookieAuth)

			apiKeyGroup := miner.Group("/api-key")
			apiKeyGroup.Use(GeneralRateLimiter())
			{
				apiKeyGroup.GET("/list", MinerCookieAuthMiddleware(), MinerApiKeyListController)
				apiKeyGroup.POST("/generate", MinerCookieAuthMiddleware(), MinerApiKeyGenerateController)
				apiKeyGroup.PUT("/disable", MinerCookieAuthMiddleware(), MinerApiKeyDisableController)
			}

			subScriptionKeyGroup := miner.Group("/subscription-key")
			subScriptionKeyGroup.Use(GeneralRateLimiter())
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
			metrics.GET("/completed-tasks-by-interval", GetCompletedTasksCountByIntervalController)
		}
		analytics := apiV1.Group("/analytics")
		{
			analytics.GET("/task-analytics-list", AthenaReadRateLimiter(), GetAnalyticsTaskListController)
			analytics.GET("/task-analytics/:taskId", AthenaAnalyticsRateLimiter(), GetAnalyticsTaskItemByIdController)
		}
	}
}
