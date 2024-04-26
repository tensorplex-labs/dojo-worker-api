package api

import (
	"dojo-api/db"
	"dojo-api/pkg/orm"
	"dojo-api/pkg/task"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
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
	_, alreadyExists := db.IsErrUniqueConstraint(err)
	if err != nil {
		if !alreadyExists {
			log.Error().Err(err).Msg("Failed to create worker")
			c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to create worker"))
			return
		} else {
			log.Warn().Err(err).Msg("Worker already exists")
		}
	}
	log.Info().Str("walletAddress", walletAddress).Str("alreadyExists", fmt.Sprintf("%+v", alreadyExists)).Msg("Worker created successfully or already exists")
	c.JSON(http.StatusOK, defaultSuccessResponse(token))
}

// POST /api/v1/tasks
func CreateTaskController(c *gin.Context) {
	var requestBody task.TaskRequest
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

	err := validateTaskRequest(requestBody)
	if err != nil {
		response["error"] = fmt.Sprintf("Error validating request: %v", err)
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

func validateTaskRequest(taskData task.TaskRequest) error {
	if taskData.Title == "" {
		return errors.New("title is required")
	}

	if taskData.Body == "" {
		return errors.New("body is required")
	}

	if taskData.ExpireAt == "" {
		return errors.New("expireAt is required")
	}

	for _, taskInterface := range taskData.TaskData {
		err := validateTaskData(taskInterface)
		if err != nil {
			return err
		}
	}

	if taskData.MaxResults == 0 {
		return errors.New("maxResults is required")
	}

	if taskData.TotalRewards == 0 {
		return errors.New("totalRewards is required")
	}

	return nil
}

func validateTaskData(taskData task.TaskData) error {
	if taskData.Prompt == "" && len(taskData.Dialogue) == 0 {
		return errors.New("prompt is required")
	}

	if taskData.Task == "" {
		return errors.New("task is required")
	}
	task := taskData.Task
	if task == "CODE_GENERATION" || task == "TEXT_TO_IMAGE" {
		for _, taskresponse := range taskData.Responses {
			var ok bool
			if task == "CODE_GENERATION" {
				fmt.Println(taskresponse.Completion)
				_, ok = taskresponse.Completion.(map[string]interface{})
			} else if task == "TEXT_TO_IMAGE" {
				_, ok = taskresponse.Completion.(string)
			}
			if !ok {
				return fmt.Errorf("invalid completion format: %v", taskresponse.Completion)
			}
		}

		if len(taskData.Dialogue) != 0 {
			return errors.New("dialogue should be empty for code generation and text to image tasks")
		}
		// TODO: change to dialogue when schema is updated
	} else if task == "CONVERSATION" {
		if len(taskData.Responses) != 0 {
			return errors.New("responses should be empty for dialogue task")
		}

		if len(taskData.Dialogue) == 0 {
			return errors.New("dialogue is required for dialogue task")
		}
	} else {
		return errors.New("invalid task")
	}

	if len(taskData.Criteria) == 0 {
		return errors.New("criteria is required")
	}

	for _, criteria := range taskData.Criteria {
		if criteria.Type == "" {
			return errors.New("type is required for criteria")
		}

		if criteria.Type == "multi-select" || criteria.Type == "ranking" {
			if len(criteria.Options) == 0 {
				return errors.New("options is required for multiple choice criteria")
			}
		} else if criteria.Type == "score" {
			if criteria.Min == 0 && criteria.Max == 0 {
				return errors.New("min or max is required for numeric criteria")
			}
		}
	}

	return nil
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

func MinerController(c *gin.Context) {
	apiKey := c.Request.Header.Get("X-API-KEY")

	minerUserORM := orm.NewMinerUserORM()
	minerUser, _ := minerUserORM.GetByApiKey(apiKey)
	if minerUser == nil {
		c.JSON(http.StatusNotFound, defaultErrorResponse("miner not found"))
		return
	}

	log.Info().Str("minerUser", fmt.Sprintf("%+v", minerUser)).Msg("Miner user found")
	c.JSON(http.StatusOK, defaultSuccessResponse(map[string]string{
		"minerId": minerUser.ID,
	}))
}

func WorkerPartnerController(c *gin.Context) {
	jwtClaims, ok := c.Get("userInfo")
	if !ok {
		log.Error().Str("userInfo", fmt.Sprintf("%+v", jwtClaims)).Msg("No user info found in context")
		c.JSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
		return
	}

	userInfo, ok := jwtClaims.(*jwt.RegisteredClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
		return
	}
	worker, err := orm.NewDojoWorkerService().GetDojoWorkerByWalletAddress(userInfo.Subject)
	if err != nil {
		c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to get worker"))
		return
	}

	var requestBody struct {
		MinerId string `json:"minerId"`
	}

	if err := c.BindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Invalid request body"))
		return
	}

	_, err = orm.NewWorkerPartnerORM().Create(worker.ID, requestBody.MinerId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, defaultErrorResponse(err.Error()))
		return
	}

	c.JSON(http.StatusOK, defaultSuccessResponse("successfully created worker-miner partnership"))
}
