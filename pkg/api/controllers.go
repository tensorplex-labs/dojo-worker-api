package api

import (
	"context"
	"dojo-api/db"
	"fmt"
	"time"
	"encoding/json"
	"github.com/gin-gonic/gin"
	// "github.com/steebchen/prisma-client-go/runtime/types"
)

type TaskData struct {
    Title       string      `json:"title"`
    Body        string      `json:"body"`
    ExpireAt    time.Time   `json:"expireAt"`
    TaskData    []map[string]interface{} `json:"taskData"`
    MaxResults  int         `json:"maxResults"`
    TotalRewards float64    `json:"totalRewards"`
}

func CreateTaskController(c *gin.Context) {
	var requestBody map[string]interface{}
	var logger = GetLogger()
	networkUserId, exists := c.Get("networkUserID")
	if !exists {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	if err := c.BindJSON(&requestBody); err != nil {
		// DO SOMETHING WITH THE ERROR
		logger.Error().Msg(fmt.Sprintf("Error binding request body: %v", err))
	}
	createTask(requestBody, networkUserId.(string), c)
}

func createTask(taskData map[string]interface{}, userid string, c *gin.Context){
	var logger = GetLogger()
	client := db.NewClient()
	ctx := context.Background()
	defer func() {
		if err := client.Prisma.Disconnect(); err != nil {
			logger.Error().Msgf("Error disconnecting: %v", err)
		}
	}()
	client.Prisma.Connect()
	var numtasks = 0
	for _, taskInterface := range taskData["taskData"].([]interface{}) {
		task, ok := taskInterface.(map[string]interface{})
		if !ok {
            logger.Error().Msg("Invalid task data format")
            c.JSON(400, gin.H{"error": "Invalid task data format"})
            return
        }
        expireAtStr, ok := taskData["expireAt"].(string)
        if !ok {
            logger.Error().Msg("Missing or invalid expireAt field")
			fmt.Println(task["expireAt"])
            c.JSON(400, gin.H{"error": "Missing or invalid expireAt field"})
            return
        }
        parsedTime, err := time.Parse(time.DateTime, expireAtStr)
        if err != nil {
            logger.Error().Msgf("Error parsing time: %v", err)
            c.JSON(400, gin.H{"error": "Invalid time format"})
            return
        }
		modalityStr, ok := task["task"].(string)
        if !ok {
            logger.Error().Msg("Missing or invalid modality field")
			fmt.Println(task)
            c.JSON(400, gin.H{"error": "Missing or invalid modality field"})
            return
        }
        modality := db.TaskModality(modalityStr)

		criteriaJSON, err := json.Marshal(task["criteria"])
        if err != nil {
            logger.Error().Msgf("Error marshaling criteria: %v", err)
            c.JSON(400, gin.H{"error": "Invalid criteria format"})
            return
        }

		taskInfoJSON, err := json.Marshal(task["taskData"])
		if err != nil {
			logger.Error().Msgf("Error marshaling task data: %v", err)
			c.JSON(400, gin.H{"error": "Invalid task data format"})
			return
		}
		fmt.Println(taskData["maxResults"])

		_, err = client.Task.CreateOne(
			db.Task.Title.Set(taskData["title"].(string)),
			db.Task.Body.Set(taskData["body"].(string)),
			db.Task.Modality.Set(modality),
			db.Task.ExpireAt.Set(parsedTime),
			db.Task.Criteria.Set(criteriaJSON),
			db.Task.TaskData.Set(taskInfoJSON),
			db.Task.Status.Set("PENDING"),
			db.Task.MaxResults.Set(int(taskData["maxResults"].(float64))),
			db.Task.NumResults.Set(0),
			db.Task.TotalYield.Set(taskData["totalRewards"].(float64)),
			db.Task.NetworkUser.Link(
				db.NetworkUser.ID.Equals(userid),
			),
		).Exec(ctx)

		if err != nil {
			logger.Error().Msgf("Error creating task: %v", err)
			c.JSON(500, gin.H{"error": "Error creating task"})
			return
		}

		numtasks++
	}

	c.JSON(200, gin.H{"success": fmt.Sprintf("Created %d tasks", numtasks)})
}
