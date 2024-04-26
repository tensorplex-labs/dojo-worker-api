package api

import (
    "github.com/gin-gonic/gin"
    "dojo-api/pkg/orm"
    "net/http"
    "github.com/rs/zerolog/log"
	"dojo-api/utils"
	"dojo-api/pkg/task"
	"errors"
	"fmt"
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

func CreateTaskController(c *gin.Context) {
	var requestBody utils.TaskRequest
	var logger = utils.GetLogger()
	response := make(map[string]interface{})
	response["success"] = false
	response["body"] = nil
	networkUserId, exists := c.Get("networkUserID")
	if !exists {
		response["error"] = "Unauthorized"
		c.JSON(401, response)
		return
	}

	if err := c.BindJSON(&requestBody); err != nil {
		// DO SOMETHING WITH THE ERROR
		logger.Error().Msg(fmt.Sprintf("Error binding request body: %v", err))
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
	msg, err := taskService.CreateTask(requestBody, networkUserId.(string))

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

func validateTaskRequest(taskData utils.TaskRequest) error {
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

func validateTaskData(taskData utils.TaskData) error {
	if taskData.Prompt == "" && len(taskData.Dialogue) == 0{
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
	}else {
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