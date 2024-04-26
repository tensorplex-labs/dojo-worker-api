package api

import (
	"dojo-api/pkg/task"
	"dojo-api/utils"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
)

// POST /api/v1/tasks
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
		response["success"] = false
		response["body"] = nil
		response["error"] = fmt.Sprintf("Error binding request body: %v", err)
		c.JSON(400, response)
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

//{
//  "taskId": "Unique Task ID",
//  "dojoWorkerId": "Unique Dojo Worker ID", //no need will get from jwt
//  "resultData": {},
//}

type WorkerTask struct {
	TaskId string `json:"taskId"`
	DojoWorkerId string `json:"dojoWorkerId"`
	ResultData map[string]interface{} `json:"resultData"`

}


// PUT/api/v1/tasks/{task-id}
func SubmitWorkerTaskController(c *gin.Context) {
	var requestBody WorkerTask
	var logger = utils.GetLogger()

    // Get the task id from the path
	c.Bind(&requestBody)

	// Validate the request body for required fields [taskId, dojoWorkerId, resultData]
	// TODO: Implement validation for the request body


	// Get the task id and dojo worker id from the request
	dojoWorkerId := requestBody.DojoWorkerId
    taskId := c.Param("task-id")
	
	logger.Info().Msg(fmt.Sprintf("Dojo Worker ID: %v", dojoWorkerId))
	logger.Info().Msg(fmt.Sprintf("Task ID: %v", taskId))
	
	task_service := task.NewTaskService()
    // Get a context.Context object from the gin context
    ctx := c.Request.Context()

    // Get corresponding task and dojoworker data
    worker, err := task_service.GetDojoWorkerById(ctx, dojoWorkerId)
    if err != nil {
        logger.Error().Msg(fmt.Sprintf("Error getting DojoWorker: %v", err))
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
	// print the worker data
	logger.Info().Msg(fmt.Sprintf("Dojo Worker by id Data pulled: %v", worker))

    task, err := task_service.GetTaskById(ctx, taskId)
    if err != nil {
        logger.Error().Msg(fmt.Sprintf("Error getting Task: %v", err))
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
	// print the task data
	logger.Info().Msg(fmt.Sprintf("Task Data by id pulled: %v", task))

	// Update the task with the result data
	numResults, err := task_service.UpdateTaskResultData(ctx, taskId,dojoWorkerId ,requestBody.ResultData)
	if err != nil {
		logger.Error().Msg(fmt.Sprintf("Error updating task: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Response payload example
	//{
	//"success": "true",
	//"body": {
	//	"numResults": 3
	//},
	//"error": null
	//}

    c.JSON(http.StatusOK, gin.H{"success": true, "body": gin.H{"numResults": numResults}, "error": nil})

}