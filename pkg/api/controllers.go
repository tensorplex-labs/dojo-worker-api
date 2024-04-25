package api

import (
	"dojo-api/pkg/task"
	"dojo-api/utils"
	"fmt"
	"github.com/gin-gonic/gin"
)

func CreateTaskController(c *gin.Context) {
	var requestBody map[string]interface{}
	var logger = utils.GetLogger()
	response := make(map[string]interface{})
	networkUserId, exists := c.Get("networkUserID")
	if !exists {
		response["success"] = false
		response["body"] = nil
		response["error"] = "Unauthorized"
		c.JSON(401, response)
		return
	}

	if err := c.BindJSON(&requestBody); err != nil {
		// DO SOMETHING WITH THE ERROR
		logger.Error().Msg(fmt.Sprintf("Error binding request body: %v", err))
	}
	taskService := task.NewTaskService()
	msg, err := taskService.CreateTask(requestBody, networkUserId.(string))

	if err != nil {
		response["success"] = false
		response["body"] = nil
		response["error"] = fmt.Sprintf("Error creating task: %v", err)
		c.JSON(500, response)
		return
	}

	response["success"] = true
	response["body"] = msg
	response["error"] = nil
	c.JSON(200, response)
}


