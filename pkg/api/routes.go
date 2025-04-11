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
			worker.Use(WorkerRateLimiter())
			worker.POST("/login/auth", WorkerLoginMiddleware(), ResourceProfiler(), WorkerLoginController)
			worker.POST("/partner", WorkerAuthMiddleware(), ResourceProfiler(), WorkerPartnerCreateController)
			worker.PUT("/partner/disable", WorkerAuthMiddleware(), ResourceProfiler(), DisableMinerByWorkerController)
			worker.GET("/partner/list", WorkerAuthMiddleware(), ResourceProfiler(), GetWorkerPartnerListController)
		}
		apiV1.GET("/auth/:address", GeneralRateLimiter(), ResourceProfiler(), GenerateNonceController)
		apiV1.PUT("/partner/edit", GeneralRateLimiter(), ResourceProfiler(), WorkerAuthMiddleware(), UpdateWorkerPartnerController)
		tasks := apiV1.Group("/tasks")
		{
			tasks.PUT("/submit-result/:task-id", WorkerAuthMiddleware(), ResourceProfiler(), SubmitTaskResultController)
			// TODO: re-enable InMetagraphOnly(), and rate limiter in future
			tasks.POST("/create-tasks", MinerAuthMiddleware(), ResourceProfiler(), CreateTasksController)
			tasks.GET("/task-result/:task-id", ReadTaskRateLimiter(), ResourceProfiler(), GetTaskResultsController)
			tasks.GET("/:task-id", ReadTaskRateLimiter(), ResourceProfiler(), GetTaskByIdController)
			tasks.GET("/next-task/:task-id", ReadTaskRateLimiter(), WorkerAuthMiddleware(), ResourceProfiler(), GetNextInProgressTaskController)
			tasks.GET("/", ReadTaskRateLimiter(), WorkerAuthMiddleware(), ResourceProfiler(), GetTasksByPageController)
		}

		miner := apiV1.Group("/miner")
		{
			miner.POST("/session/auth", GeneralRateLimiter(), ResourceProfiler(), GenerateCookieAuth)

			apiKeyGroup := miner.Group("/api-key")
			apiKeyGroup.Use(GeneralRateLimiter())
			{
				apiKeyGroup.GET("/list", MinerCookieAuthMiddleware(), ResourceProfiler(), MinerApiKeyListController)
				apiKeyGroup.POST("/generate", MinerCookieAuthMiddleware(), ResourceProfiler(), MinerApiKeyGenerateController)
				apiKeyGroup.PUT("/disable", MinerCookieAuthMiddleware(), ResourceProfiler(), MinerApiKeyDisableController)
			}

			subScriptionKeyGroup := miner.Group("/subscription-key")
			subScriptionKeyGroup.Use(GeneralRateLimiter())
			{
				subScriptionKeyGroup.GET("/list", MinerCookieAuthMiddleware(), ResourceProfiler(), MinerSubscriptionKeyListController)
				subScriptionKeyGroup.POST("/generate", MinerCookieAuthMiddleware(), ResourceProfiler(), MinerSubscriptionKeyGenerateController)
				subScriptionKeyGroup.PUT("/disable", MinerCookieAuthMiddleware(), ResourceProfiler(), MinerSubscriptionKeyDisableController)
			}
		}
		metrics := apiV1.Group("/metrics")
		{
			metrics.Use(MetricsRateLimiter())
			metrics.GET("/dojo-worker-count", ResourceProfiler(), GetDojoWorkerCountController)
			metrics.GET("/completed-tasks-count", ResourceProfiler(), GetTotalCompletedTasksController)
			metrics.GET("/task-result-count", ResourceProfiler(), GetTotalTasksResultsController)
			metrics.GET("/average-task-completion-time", ResourceProfiler(), GetAvgTaskCompletionTimeController)
			metrics.GET("/completed-tasks-by-interval", ResourceProfiler(), GetCompletedTasksCountByIntervalController)
		}
	}
}
