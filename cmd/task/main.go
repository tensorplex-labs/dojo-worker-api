package main

import (
	"dojo-api/pkg/task"
	"dojo-api/utils"
	"fmt"
	"log"
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

	// Define the GET endpoint
	taskService := task.NewTaskService()

	router.GET("/api/v1/get-task/:task-id", func(c *gin.Context) {
		taskID := c.Param("task-id")
		task, err := taskService.GetTaskById(c.Request.Context(), taskID)

		if err != nil {
			// utils.(c, http.StatusNotFound, http.StatusNotFound, err.Error())
			log.Println(err)
			utils.ErrorHandler(c, http.StatusNotFound, err.Error())
			return
		}

		if task == nil {
			utils.ErrorHandler(c, http.StatusAccepted, "Task not found")
			return
		}

		// Successful response
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    task,
		})
	})

	router.Run(port)
}
