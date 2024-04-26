package api

import (
    "github.com/gin-gonic/gin"
    "dojo-api/pkg/orm"
    "net/http"
    "github.com/rs/zerolog/log"
	"dojo-api/utils"
	"dojo-api/pkg/task"
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

	if err := ValidateTaskRequest(requestBody); err != nil {
		response["error"] = fmt.Sprintf("Error validating request: %v", err)
		c.JSON(400, response)
		return
	}

	if err := ProcessTaskRequest(&requestBody); err != nil{
		response["error"] = fmt.Sprintf("Error processing request: %v", err)
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


