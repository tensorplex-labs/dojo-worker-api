package api

import (
	"github.com/gin-gonic/gin"
)

func LoginRoutes(router *gin.Engine) {

	// Grouping routes
	apiGroup := router.Group("/api")
    {
    	apiGroup.POST("/v1/tasks/", UserAuthMiddleware(), CreateTaskController)
    }
	// Submit a task response by a worker.
	{
		apiGroup.PUT("/v1/tasks/:task-id", SubmitWorkerTaskController)
	}
}