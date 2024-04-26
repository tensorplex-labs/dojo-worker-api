package api

import (
	"github.com/gin-gonic/gin"
)

func LoginRoutes(router *gin.Engine) {

	// Grouping routes
	workerApiGroup := router.Group("/api/v1")
    {
    	workerApiGroup.POST("/login/auth", LoginMiddleware(), LoginController)
		workerApiGroup.POST("/tasks/", UserAuthMiddleware(), CreateTaskController)
    }
}
 
