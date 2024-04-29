package api

import (
	"dojo-api/pkg/orm"
	"dojo-api/pkg/task"
	"dojo-api/utils"
	"fmt"
	"net/http"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func LoginController(c *gin.Context) {
	walletAddressInterface, _ := c.Get("WalletAddress")
	chainIdInterface, _ := c.Get("ChainId")
	token, _ := c.Get("JWTToken")

	walletAddress, ok := walletAddressInterface.(string)
	if !ok {
		log.Error().Msg("Invalid wallet address provided")
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Invalid wallet address"))
		return
	}
	chainId, ok := chainIdInterface.(string)
	if !ok {
		log.Error().Msg("Invalid signature provided")
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Invalid signature"))
		return
	}

	workerService := orm.NewDojoWorkerService()
	_, err := workerService.CreateDojoWorker(walletAddress, chainId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create worker")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to create worker"))
		return
	}
	log.Info().Str("walletAddress", walletAddress).Msg("Worker created successfully")
	c.JSON(http.StatusOK, defaultSuccessResponse(token))
}

// POST /api/v1/tasks
func CreateTaskController(c *gin.Context) {
	var requestBody utils.TaskRequest
	response := make(map[string]interface{})
	response["success"] = false
	response["body"] = nil
	minerUserId, exists := c.Get("minerUserID")
	if !exists {
		response["error"] = "Unauthorized"
		c.JSON(401, response)
		return
	}

	if err := c.BindJSON(&requestBody); err != nil {
		// DO SOMETHING WITH THE ERROR
		log.Error().Msg(fmt.Sprintf("Error binding request body: %v", err))
		response["error"] = fmt.Sprintf("Error binding request body: %v", err)
		c.JSON(400, response)
	}

	if err := task.ValidateTaskRequest(requestBody); err != nil {
		response["error"] = fmt.Sprintf("Error validating request: %v", err)
		c.JSON(400, response)
		return
	}

	if err := task.ProcessTaskRequest(&requestBody); err != nil{
		response["error"] = fmt.Sprintf("Error processing request: %v", err)
		c.JSON(400, response)
		return
	}

	taskService := task.NewTaskService()
	msg, err := taskService.CreateTask(requestBody, minerUserId.(string))

	if err != nil {
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
	TaskId       string                 `json:"taskId"`
	DojoWorkerId string                 `json:"dojoWorkerId"`
	ResultData   map[string]interface{} `json:"resultData"`
}

// PUT/api/v1/tasks/{task-id}
func SubmitWorkerTaskController(c *gin.Context) {
	var requestBody WorkerTask

	// Get the task id from the path
	c.Bind(&requestBody)

	// Validate the request body for required fields [taskId, dojoWorkerId, resultData]
	// TODO: Implement validation for the request body

	// Get the task id and dojo worker id from the request
	dojoWorkerId := requestBody.DojoWorkerId
	taskId := c.Param("task-id")

	log.Info().Msg(fmt.Sprintf("Dojo Worker ID: %v", dojoWorkerId))
	log.Info().Msg(fmt.Sprintf("Task ID: %v", taskId))

	task_service := task.NewTaskService()
	// Get a context.Context object from the gin context
	ctx := c.Request.Context()

	// Get corresponding task and dojoworker data
	worker, err := task_service.GetDojoWorkerById(ctx, dojoWorkerId)
	if err != nil {
		log.Error().Msg(fmt.Sprintf("Error getting DojoWorker: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// print the worker data
	log.Info().Msg(fmt.Sprintf("Dojo Worker by id Data pulled: %v", worker))

	task, err := task_service.GetTaskById(ctx, taskId)
	if err != nil {
		log.Error().Msg(fmt.Sprintf("Error getting Task: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// print the task data
	log.Info().Msg(fmt.Sprintf("Task Data by id pulled: %v", task))

	// Update the task with the result data
	numResults, err := task_service.UpdateTaskResultData(ctx, taskId, dojoWorkerId, requestBody.ResultData)
	if err != nil {
		log.Error().Msg(fmt.Sprintf("Error updating task: %v", err))
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
