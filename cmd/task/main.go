package main

import (
	"dojo-api/pkg/task"
	"dojo-api/utils"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	port := os.Getenv("PORT")
	fmt.Println("Using port:", port)
	if port == "" {
		port = "4001" // default to 4001 if no environment variable is set
	}
	port = ":" + port

	router := gin.Default()

	// Hello World
	router.GET("/hello-world", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Hello World",
		})
	})

	// Task creation endpoint
	// router.POST("/api/v1/create-task", func(c *gin.Context) {

	// })

	taskService := task.NewTaskService()

	router.GET("/api/v1/tasks/:task-id", func(c *gin.Context) {
		taskID := c.Param("task-id")
		task, err := taskService.GetTaskById(c.Request.Context(), taskID)

		if err != nil {
			utils.ErrorHandler(c, http.StatusInternalServerError, "Internal server error")
			return
		}

		if task == nil {
			utils.ErrorHandler(c, http.StatusNotFound, "Task not found")
			return
		}

		// Successful response
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    task,
		})
	})

	router.Run(port) // Default listens on :8080
}
