package api

import (
	"errors"
	"fmt"
	"net/http"

	"dojo-api/db"
	"dojo-api/pkg/orm"
	"dojo-api/pkg/task"

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
		log.Error().Msg("Invalid chainId provided")
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Invalid chainId"))
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
		}
		log.Warn().Err(err).Msg("Worker already exists")
	}
	log.Info().Str("walletAddress", walletAddress).Str("alreadyExists", fmt.Sprintf("%+v", alreadyExists)).Msg("Worker created successfully or already exists")
	c.JSON(http.StatusOK, defaultSuccessResponse(token))
}

func CreateTaskController(c *gin.Context) {
	minerUserId, exists := c.Get("minerUserID")
	if !exists {
		c.JSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
		c.Abort()
		return
	}

	var requestBody task.CreateTaskRequest
	if err := c.BindJSON(&requestBody); err != nil {
		log.Error().Err(err).Msg("Invalid request body")
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Invalid request body"))
		c.Abort()
		return
	}

	err := task.ValidateTaskRequest(requestBody)
	if err != nil {
		log.Error().Err(err).Msg("Failed to validate task request")
		c.JSON(http.StatusBadRequest, defaultErrorResponse(err.Error()))
		c.Abort()
		return
	}

	taskService := task.NewTaskService()
	taskIds, err := taskService.CreateTasks(requestBody, minerUserId.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, defaultErrorResponse(err.Error()))
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, defaultSuccessResponse(taskIds))
}

func SubmitTaskResultController(c *gin.Context) {
	var requestBody task.SubmitTaskResultRequest
	if err := c.BindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, defaultErrorResponse("Invalid request body"))
		return
	}

	// Validate the request body for required fields [taskId, dojoWorkerId, resultData]
	// TODO: Implement validation for the request body

	// Get the task id and dojo worker id from the request
	dojoWorkerId := requestBody.DojoWorkerId
	taskId := c.Param("task-id")

	log.Info().Msgf("Dojo Worker ID: %v", dojoWorkerId)
	log.Info().Msgf("Task ID: %v", taskId)

	taskService := task.NewTaskService()
	// Get a context.Context object from the gin context
	ctx := c.Request.Context()

	// Get corresponding task and dojoworker data
	worker, err := taskService.GetDojoWorkerById(ctx, dojoWorkerId)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			log.Error().Err(err).Msg("Error getting DojoWorker")
			c.JSON(http.StatusNotFound, defaultErrorResponse(err.Error()))
			return
		}

		log.Error().Err(err).Msg("Error getting DojoWorker")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse(err.Error()))
		return
	}
	// print the worker data
	log.Info().Msgf("Dojo Worker by id Data pulled: %v", worker)

	task, err := taskService.GetTaskById(ctx, taskId)
	if err != nil {
		log.Error().Err(err).Msg("Error getting Task")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse(err.Error()))
		return
	}
	// print the task data
	log.Info().Msgf("Task Data by id pulled: %v", task)

	// Update the task with the result data
	updatedTask, err := taskService.UpdateTaskResultData(ctx, taskId, dojoWorkerId, requestBody.ResultData)
	if err != nil {
		log.Error().Err(err).Msg("Error updating task")
		c.JSON(http.StatusInternalServerError, defaultErrorResponse(err.Error()))
		return
	}

	c.JSON(http.StatusOK, defaultSuccessResponse(map[string]interface{}{
		"numResults": updatedTask.NumResults,
	}))
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
