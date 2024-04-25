package api

import (
	"github.com/gin-gonic/gin"
)

func LoginRoutes(router *gin.Engine) {

	// Grouping routes
	workerApiGroup := router.Group("/api/worker")
    {
    	workerApiGroup.POST("/login/auth", LoginMiddleware(), LoginController)
    }
}
 
