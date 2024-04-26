package main

import (
	"dojo-api/pkg/task"
	"dojo-api/utils"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
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
			"body":    task,
			"error":   nil,
		})
	})

	router.GET("/api/v1/tasks", func(c *gin.Context) {

		// Get the task query parameter as a single string
		taskParam := c.Query("task")
		// Split the string into a slice of strings
		taskTypes := strings.Split(taskParam, ",")

		// Parsing "page" and "limit" as integers with default values
		pageStr := c.DefaultQuery("page", "1")
		limitStr := c.DefaultQuery("limit", "10")
		sort := c.DefaultQuery("sort", "createdAt")

		page, err := strconv.Atoi(pageStr)
		if err != nil {
			log.Error().Err(err).Msg("Error converting page to integer:")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page parameter"})
			return
		}

		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			log.Error().Err(err).Msg("Error converting page to integer:")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit parameter"})
			return
		}

		// fetching tasks by pagination
		taskPagination, err := taskService.GetTasksByPagination(c.Request.Context(), page, limit, taskTypes, sort)
		if err != nil {
			log.Error().Err(err).Msg("Error getting tasks by pagination")
			utils.ErrorHandler(c, http.StatusInternalServerError, "Internal server error")
			return
		}

		if taskPagination == nil {
			log.Error().Err(err).Msg("Error getting tasks by pagination")
			utils.ErrorHandler(c, http.StatusNotFound, "No tasks found")
			return
		}

		// Successful response
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"body": gin.H{
				"tasks": taskPagination,
			},
			"error": nil,
		})
	})

	router.Run(port) // Default listens on :8080
}
